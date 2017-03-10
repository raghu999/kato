package ns1

//-----------------------------------------------------------------------------
// Package factored import statement:
//-----------------------------------------------------------------------------

import (

	// Stdlib:
	"net/http"
	"strings"
	"time"

	// Community:
	log "github.com/Sirupsen/logrus"
	api "gopkg.in/ns1/ns1-go.v2/rest"
	"gopkg.in/ns1/ns1-go.v2/rest/model/dns"
)

//-----------------------------------------------------------------------------
// Typedefs:
//-----------------------------------------------------------------------------

// Data struct for NS1 information.
type Data struct {
	command string
	Link    string   // zone:add |
	Zones   []string // zone:add |
	APIKey  string   // zone:add | record:add
	Zone    string   //          | record:add
	Records []string //          | record:add
}

//-----------------------------------------------------------------------------
// func: AddZones
//-----------------------------------------------------------------------------

// AddZones adds one or more zones to NS1.
func (d *Data) AddZones() {

	// Set the current command:
	d.command = "zone:add"

	// Create an NS1 API client:
	httpClient := &http.Client{Timeout: time.Second * 10}
	client := api.NewClient(httpClient, api.SetAPIKey(d.APIKey))

	// For each requested zone:
	for _, zone := range d.Zones {

		// New zone handler:
		z := dns.NewZone(zone)
		if d.Link != "" {
			z.LinkTo(d.Link)
		}

		// Send the new zone request:
		if _, err := client.Zones.Create(z); err != nil {
			if err != api.ErrZoneExists {
				log.WithFields(log.Fields{"cmd": "ns1:" + d.command, "id": zone}).Fatal(err)
			}
		}

		// Log zone creation:
		log.WithFields(log.Fields{"cmd": "ns1:" + d.command, "id": zone}).
			Info("New DNS zone created")
	}
}

//-----------------------------------------------------------------------------
// func: AddRecords
//-----------------------------------------------------------------------------

// AddRecords adds one or more records to an NS1 zone.
func (d *Data) AddRecords() {

	// Set the current command:
	d.command = "record:add"

	// Create an NS1 API client:
	httpClient := &http.Client{Timeout: time.Second * 10}
	client := api.NewClient(httpClient, api.SetAPIKey(d.APIKey))

	// For each requested record:
	for _, record := range d.Records {

		// New record handler:
		s := strings.Split(record, ":")
		r := dns.NewRecord(d.Zone, s[2], s[1])
		a := dns.NewAv4Answer(s[0])
		r.AddAnswer(a)

		// Send the new record request:
		if _, err := client.Records.Create(r); err != nil {
			if err != api.ErrRecordExists {
				log.WithFields(log.Fields{"cmd": "ns1:" + d.command, "id": record}).Fatal(err)
			}
		}

		// Log record creation:
		log.WithFields(log.Fields{"cmd": "ns1:" + d.command, "id": record}).
			Info("New DNS record created")
	}
}