# Port Tester

## Purpose
The purpose of this port tester is to verify tcp communications between different tiers and services of a Morpheus installation.

For example, the application nodes need to be able to access the transactional database nodes for MySQL on port 3306.  The clustered MySQL database nodes, which use percona, need to communicate with each other on ports 4444, 4567, 4568.  This tester will verify that communication.

## Requirements

The releases have 3 files: `porttest`, `receiver`, and `sender`.  All 3 must be in the dirrectory you are rujnning `porttest` from in order to be copied to the remotes.

You must have SSH access to each remote machine.

## Usage

`porttest -c <config file>` is the basic usage.  

Additional options are available by running `porttest --help`

## Flow

1. Create SSH connections to all servers in configuration file.
2. Copy `sender` and `receiver` executables into `~/.morpheustesting` on remotes.
3. Run receiver for needed port on destination server, run sender on source server and verify communication. Receiver will automatically terminate after 10 seconds.
4. Report findings for all server and port pairs.

## Config File

The config file is yaml and follows the format:
```yaml
servers:
- name: Server1
  ip: 192.168.1.100
  appnode: true
  rabbitnode: true
  elasticnode: true
  databasenode: false
  perconanode: true
- name: Server2
  ip: 192.168.1.101
  appnode: false
  rabbitnode: false
  elasticnode: false
  databasenode: true
  perconanode: false
```

You can generate a sample file by running `porttest --generateconfig <filename>`