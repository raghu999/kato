require 'fileutils'

Vagrant.require_version ">= 1.6.0"

#------------------------------------------------------------------------------
# Variables:
#------------------------------------------------------------------------------

$master_count   = ENV['KATO_MASTER_COUNT'] || 1
$node_count     = ENV['KATO_NODE_COUNT'] || 1
$edge_count     = ENV['KATO_EDGE_COUNT'] || 0
$master_cpus    = ENV['KATO_MASTER_CPUS'] || 1
$master_memory  = ENV['KATO_MASTER_MEMORY'] || 1024
$node_cpus      = ENV['KATO_NODE_CPUS'] || 2
$node_memory    = ENV['KATO_NODE_MEMORY'] || 2048
$edge_cpus      = ENV['KATO_EDGE_CPUS'] || 1
$edge_memory    = ENV['KATO_EDGE_MEMORY'] || 512
$coreos_channel = ENV['KATO_COREOS_CHANNEL'] || 'alpha'
$coreos_version = ENV['KATO_COREOS_VERSION'] || 'current'
$ns1_api_key    = ENV['KATO_NS1_API_KEY'] || 'x'
$domain         = ENV['KATO_DOMAIN'] || 'cell-1.dc-1.demo.lan'
$ca_cert        = ENV['KATO_CA_CERT']
$box_url        = "https://storage.googleapis.com/%s.release.core-os.net/amd64-usr/%s/coreos_production_vagrant.json"
$discovery_url  = "https://discovery.etcd.io/new?size=%s"

#------------------------------------------------------------------------------
# Forge the katoctl command:
#------------------------------------------------------------------------------

$katoctl = "katoctl udata " +
  "--rexray-storage-driver virtualbox " +
  "--rexray-endpoint-ip 172.17.8.1 " +
  "--flannel-backend host-gw " +
  "--master-count %s " +
  "--ns1-api-key %s " +
  "--domain %s " +
  "--hostid %s " +
  "--role %s " +
  "--etcd-token %s " +
  "--gzip-udata"

#------------------------------------------------------------------------------
# Generate a new etcd discovery token:
#------------------------------------------------------------------------------

if $discovery_url && ARGV[0].eql?('up')
  require 'open-uri'
  token = open($discovery_url % $master_count).read.split("/")[-1]
end

#------------------------------------------------------------------------------
# Configure:
#------------------------------------------------------------------------------

Vagrant.configure("2") do |config|

  config.ssh.forward_agent = true
  config.ssh.insert_key = false
  config.vm.box = "coreos-%s" % $coreos_channel
  config.vm.box_url = $box_url % [$coreos_channel, $coreos_version]

  if ARGV[0].eql?('up')
    config.vm.provider :virtualbox do |vb|
      vb.customize ["setproperty", "websrvauthlibrary", "null"]
      vb.check_guest_additions = false
      vb.functional_vboxsf = false
      if `pgrep vboxwebsrv` == ''
        `vboxwebsrv -H 0.0.0.0 -b`
      end
    end
  end

  #----------------------------
  # Start master_count masters
  #----------------------------

  (1..$master_count.to_i).each do |i|

    config.vm.define vm_name = "master-%d" % i do |conf|

      conf.vm.hostname = "master-%d.%s" % [i, $domain]

      conf.vm.provider :virtualbox do |vb|
        vb.gui = false
        vb.name = "master-%d.%s" % [i, $domain]
        vb.memory = $master_memory
        vb.cpus = $master_cpus
        vb.customize ["modifyvm", :id, "--macaddress1", "auto" ]
      end

      ip_pri = "172.17.8.#{i+10}"
      conf.vm.network :private_network, ip: ip_pri

      if ARGV[0].eql?('up')

        if $ca_cert
          cmd = $katoctl + " -c %s > user_data_master-%s"
          system cmd % [$master_count, $ns1_api_key, $domain, i, 'master', token, $ca_cert, i ]
        else
          cmd = $katoctl + " > user_data_master-%s"
          system cmd % [$master_count, $ns1_api_key, $domain, i, 'master', token, i ]
        end

        if File.exist?("user_data_master-%s" % i)
          conf.vm.provision :file, :source => "user_data_master-%s" % i, :destination => "/tmp/vagrantfile-user-data"
          conf.vm.provision :shell, :inline => "mv /tmp/vagrantfile-user-data /var/lib/coreos-vagrant/", :privileged => true
        end

      end
    end
  end

  #------------------------
  # Start node_count nodes
  #------------------------

  (1..$node_count.to_i).each do |i|

    config.vm.define vm_name = "node-%d" % i do |conf|

      conf.vm.hostname = "node-%d.%s" % [i, $domain]

      conf.vm.provider :virtualbox do |vb|
        vb.gui = false
        vb.name = "node-%d.%s" % [i, $domain]
        vb.memory = $node_memory
        vb.cpus = $node_cpus
        vb.customize ["modifyvm", :id, "--macaddress1", "auto" ]
        if `VBoxManage showvminfo #{vb.name} 2>/dev/null | grep SATA` == ''
          vb.customize ["storagectl", :id, "--name", "SATA", "--add", "sata"]
        end
      end

      ip_pri = "172.17.8.#{i+20}"
      conf.vm.network :private_network, ip: ip_pri

      if ARGV[0].eql?('up')

        if $ca_cert
          cmd = $katoctl + " -c %s > user_data_node-%s"
          system cmd % [$master_count, $ns1_api_key, $domain, i, 'node', token, $ca_cert, i ]
        else
          cmd = $katoctl + " > user_data_node-%s"
          system cmd % [$master_count, $ns1_api_key, $domain, i, 'node', token, i ]
        end

        if File.exist?("user_data_node-%s" % i)
          conf.vm.provision :file, :source => "user_data_node-%s" % i, :destination => "/tmp/vagrantfile-user-data"
          conf.vm.provision :shell, :inline => "mv /tmp/vagrantfile-user-data /var/lib/coreos-vagrant/", :privileged => true
        end

      end
    end
  end

  #------------------------
  # Start edge_count edges
  #------------------------

  (1..$edge_count.to_i).each do |i|

    config.vm.define vm_name = "edge-%d" % i do |conf|

      conf.vm.hostname = "edge-%d.%s" % [i, $domain]

      conf.vm.provider :virtualbox do |vb|
        vb.gui = false
        vb.name = "edge-%d.%s" % [i, $domain]
        vb.memory = $edge_memory
        vb.cpus = $edge_cpus
        vb.customize ["modifyvm", :id, "--macaddress1", "auto" ]
      end

      ip_pri = "172.17.8.#{i+30}"
      conf.vm.network :private_network, ip: ip_pri

      if ARGV[0].eql?('up')

        if $ca_cert
          cmd = $katoctl + " -c %s > user_data_edge-%s"
          system cmd % [$master_count, $ns1_api_key, $domain, i, 'edge', token, $ca_cert, i ]
        else
          cmd = $katoctl + " > user_edata_%s"
          system cmd % [$master_count, $ns1_api_key, $domain, i, 'edge', token, i ]
        end

        if File.exist?("user_edata_%s" % i)
          conf.vm.provision :file, :source => "user_edata_%s" % i, :destination => "/tmp/vagrantfile-user-data"
          conf.vm.provision :shell, :inline => "mv /tmp/vagrantfile-user-data /var/lib/coreos-vagrant/", :privileged => true
        end

      end
    end
  end
end

# vi: set ft=ruby :
