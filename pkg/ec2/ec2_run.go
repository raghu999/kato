package ec2

//-----------------------------------------------------------------------------
// Package factored import statement:
//-----------------------------------------------------------------------------

import (

	// Stdlib:
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	// Community:
	log "github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/elb"
)

//-----------------------------------------------------------------------------
// func: Run
//-----------------------------------------------------------------------------

// Run uses EC2 API to launch a new instance.
func (d *Data) Run() {

	// Set current command:
	d.command = "run"

	// Read udata from stdin:
	udata, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		log.WithField("cmd", "ec2:"+d.command).Fatal(err)
	}

	// Connect and authenticate to the API endpoints:
	d.ec2 = ec2.New(session.New(&aws.Config{Region: aws.String(d.Region)}))
	d.elb = elb.New(session.New(&aws.Config{Region: aws.String(d.Region)}))

	// Run the EC2 instance:
	if err := d.runInstance(udata); err != nil {
		log.WithField("cmd", "ec2:"+d.command).Fatal(err)
	}

	// Modify instance attributes:
	if err := d.modifyInstanceAttribute(); err != nil {
		log.WithField("cmd", "ec2:"+d.command).Fatal(err)
	}

	// Setup an elastic IP:
	if d.PublicIP == "elastic" {
		if err := d.setupElasticIP(); err != nil {
			log.WithField("cmd", "ec2:"+d.command).Fatal(err)
		}
	}

	// Register with ELB:
	if d.ELBName != "" {
		if err := d.registerWithELB(); err != nil {
			log.WithField("cmd", "ec2:"+d.command).Fatal(err)
		}
	}

	// Output IP addresses to stdout:
	if err := d.stdoutIPs(); err != nil {
		log.WithField("cmd", "ec2:"+d.command).Warning(err)
	}
}

//-----------------------------------------------------------------------------
// func: runInstance
//-----------------------------------------------------------------------------

func (d *Data) runInstance(udata []byte) error {

	// Variables:
	var resp *ec2.Reservation
	var err error

	// Forge the instance request:
	params := &ec2.RunInstancesInput{
		ImageId:           aws.String(d.AmiID),
		MinCount:          aws.Int64(1),
		MaxCount:          aws.Int64(1),
		KeyName:           aws.String(d.KeyPair),
		InstanceType:      aws.String(d.InstanceType),
		NetworkInterfaces: d.forgeNetworkInterfaces(),
		Placement: &ec2.Placement{
			AvailabilityZone: aws.String(d.Region + d.Zone),
		},
		UserData: aws.String(base64.StdEncoding.EncodeToString([]byte(udata))),
		IamInstanceProfile: &ec2.IamInstanceProfileSpecification{
			Name: aws.String(d.IAMRole),
		},
	}

	// Send the instance request:
	for i := 0; i < 5; i++ {
		resp, err = d.ec2.RunInstances(params)
		if err != nil {
			ec2err, ok := err.(awserr.Error)
			if ok && strings.Contains(ec2err.Code(), "InvalidParameterValue") {
				time.Sleep(2 * time.Second)
				continue
			}
			return err
		}
		break
	}

	// Store the instance ID:
	d.InstanceID = *resp.Instances[0].InstanceId
	log.WithFields(log.Fields{"cmd": "ec2:" + d.command, "id": d.InstanceID}).
		Info("New " + d.InstanceType + " EC2 instance requested")

	// Store the interface ID:
	d.InterfaceID = *resp.Instances[0].
		NetworkInterfaces[0].NetworkInterfaceId

	// Tag the instance:
	if err := d.tag(d.InstanceID, "Name", d.TagName); err != nil {
		return err
	}

	// Pretty-print to stderr:
	log.WithFields(log.Fields{"cmd": "ec2:" + d.command, "id": d.TagName}).
		Info("New EC2 instance tagged")

	return nil
}

//-----------------------------------------------------------------------------
// func: forgeNetworkInterfaces
//-----------------------------------------------------------------------------

func (d *Data) forgeNetworkInterfaces() []*ec2.
	InstanceNetworkInterfaceSpecification {

	var networkInterfaces []*ec2.InstanceNetworkInterfaceSpecification
	var securityGroupIds []*string

	// Append to security group array:
	for _, grp := range strings.Split(d.SecGrpIDs, ",") {
		securityGroupIds = append(securityGroupIds, aws.String(grp))
	}

	// Forge the interface data type:
	iface := ec2.InstanceNetworkInterfaceSpecification{
		DeleteOnTermination: aws.Bool(true),
		DeviceIndex:         aws.Int64(int64(0)),
		Groups:              securityGroupIds,
		SubnetId:            aws.String(d.SubnetID),
	}

	// Private IP address:
	if d.PrivateIP != "" {
		iface.PrivateIpAddress = aws.String(d.PrivateIP)
	}

	// Public IP address:
	if d.PublicIP == "true" {
		iface.AssociatePublicIpAddress = aws.Bool(true)
	}

	// Append to the interfaces array:
	networkInterfaces = append(networkInterfaces, &iface)

	return networkInterfaces
}

//-----------------------------------------------------------------------------
// func: modifyInstanceAttribute
//-----------------------------------------------------------------------------

func (d *Data) modifyInstanceAttribute() error {

	// Variable transformation:
	SrcDstCheck, err := strconv.ParseBool(d.SrcDstCheck)
	if err != nil {
		return err
	}

	// Forge the attribute modification request:
	params := &ec2.ModifyInstanceAttributeInput{
		InstanceId: aws.String(d.InstanceID),
		SourceDestCheck: &ec2.AttributeBooleanValue{
			Value: aws.Bool(SrcDstCheck),
		},
	}

	// Send the attribute modification request:
	_, err = d.ec2.ModifyInstanceAttribute(params)
	if err != nil {
		return err
	}

	return nil
}

//-----------------------------------------------------------------------------
// func: setupElasticIP
//-----------------------------------------------------------------------------

func (d *Data) setupElasticIP() error {

	// Allocate an elastic IP address:
	if err := d.allocateElasticIP(); err != nil {
		return err
	}

	// Associate the elastic IP:
	if err := d.associateElasticIP(); err != nil {
		return err
	}

	return nil
}

//-----------------------------------------------------------------------------
// func: associateElasticIP
//-----------------------------------------------------------------------------

func (d *Data) associateElasticIP() error {

	// Wait until instance is running:
	if err := d.ec2.WaitUntilInstanceRunning(&ec2.DescribeInstancesInput{
		InstanceIds: []*string{aws.String(d.InstanceID)}}); err != nil {
		return err
	}

	// Forge the association request:
	params := &ec2.AssociateAddressInput{
		AllocationId:       aws.String(d.AllocationID),
		AllowReassociation: aws.Bool(true),
		NetworkInterfaceId: aws.String(d.InterfaceID),
	}

	// Send the association request:
	resp, err := d.ec2.AssociateAddress(params)
	if err != nil {
		return err
	}

	// Log to stderr:
	log.WithFields(log.Fields{
		"cmd": "ec2:" + d.command, "id": *resp.AssociationId}).
		Info("New elastic IP association")

	return nil
}

//-----------------------------------------------------------------------------
// func: registerWithELB
//-----------------------------------------------------------------------------

func (d *Data) registerWithELB() error {

	// Forge the register request:
	params := &elb.RegisterInstancesWithLoadBalancerInput{
		Instances: []*elb.Instance{
			{
				InstanceId: aws.String(d.InstanceID),
			},
		},
		LoadBalancerName: aws.String(d.ELBName),
	}

	// Send the register request:
	if _, err := d.elb.RegisterInstancesWithLoadBalancer(params); err != nil {
		return err
	}

	// Log this action:
	log.WithFields(log.Fields{"cmd": "ec2:" + d.command, "id": d.ELBName}).
		Info("Instance registered with ELB")

	return nil
}

//-----------------------------------------------------------------------------
// func: stdoutIPs
//-----------------------------------------------------------------------------

func (d *Data) stdoutIPs() error {

	// Map to store the output:
	m := make(map[string]string)

	// Forge the describe request:
	params := &ec2.DescribeNetworkInterfacesInput{
		NetworkInterfaceIds: []*string{
			aws.String(d.InterfaceID),
		},
	}

	// Retry loop:
	for i := 0; i < 5; i++ {

		// Send the describe request:
		resp, err := d.ec2.DescribeNetworkInterfaces(params)
		if err != nil {
			return err
		}

		// Extract data from response:
		if len(resp.NetworkInterfaces) > 0 && len(resp.NetworkInterfaces[0].PrivateIpAddresses) > 0 {

			// Internal IP address:
			if resp.NetworkInterfaces[0].PrivateIpAddresses[0].PrivateIpAddress != nil {
				m["internal"] = *resp.NetworkInterfaces[0].PrivateIpAddresses[0].PrivateIpAddress
			}

			// External IP address:
			if resp.NetworkInterfaces[0].PrivateIpAddresses[0].Association != nil {
				if resp.NetworkInterfaces[0].PrivateIpAddresses[0].Association.PublicIp != nil {
					m["external"] = *resp.NetworkInterfaces[0].PrivateIpAddresses[0].Association.PublicIp
				}
			}
		}

		// Sleep and try again:
		if m["internal"] == "" {
			time.Sleep(2 * time.Second)
			continue
		}

		break
	}

	// JSON encode:
	jsn, err := json.Marshal(m)
	if err != nil {
		return err
	}

	// Print and return:
	fmt.Println(string(jsn))
	return nil
}
