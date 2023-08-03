package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/melbahja/goph"
	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
	"gopkg.in/yaml.v2"
)

var (
	pass               bool
	key                string
	passphrase         bool
	accepthostkeys     bool
	auth               goph.Auth
	output             string
	configFile         string
	generateConfigFile string
)

type Server struct {
	Name         string `yaml:"name"`
	IP           string `yaml:"ip"`
	AppNode      bool   `yaml:"appnode"`
	RabbitNode   bool   `yaml:"rabbitnode"`
	ElasticNode  bool   `yaml:"elasticnode"`
	DatabaseNode bool   `yaml:"databasenode"`
	PerconaNode  bool   `yaml:"perconanode"`
}

type ServerModel struct {
	Servers []Server `yaml:"servers"`
}

type Result struct {
	Source      string
	Destination string
	Path        string
	Service     string
	Success     bool
	Error       string
}

func init() {
	flag.BoolVar(&pass, "askpass", false, "ask for ssh password")
	flag.StringVar(&key, "key", filepath.Join(os.Getenv("HOME"), ".ssh", "id_rsa"), "private key path")
	flag.BoolVar(&passphrase, "passphrase", false, "ask for private key passphrase.")
	flag.BoolVar(&accepthostkeys, "accepthostkeys", false, "accept all unknown host keys")
	flag.StringVar(&configFile, "configfile", "", "config file")
	flag.StringVar(&configFile, "c", "", "config file")
	flag.StringVar(&generateConfigFile, "generateconfig", "", "generate sample config file")
}

func main() {
	flag.Parse()

	var err error

	if generateConfigFile != "" {
		sampleData := ServerModel{
			Servers: []Server{
				{
					Name:         "Server1",
					IP:           "192.168.1.100",
					AppNode:      true,
					RabbitNode:   true,
					ElasticNode:  true,
					DatabaseNode: false,
					PerconaNode:  true,
				},
				{
					Name:         "Server2",
					IP:           "192.168.1.101",
					AppNode:      false,
					RabbitNode:   false,
					ElasticNode:  false,
					DatabaseNode: true,
					PerconaNode:  false,
				},
			},
		}

		yamlData, err := yaml.Marshal(sampleData)
		if err != nil {
			fmt.Println("Error marshaling YAML:", err)
			return
		}

		_, err = os.Stat(generateConfigFile)
		if !os.IsNotExist(err) {
			fmt.Println("File already exists, will not overwrite")
			os.Exit(1)
		}

		newConfigFile, err := os.Create(generateConfigFile)
		if err != nil {
			fmt.Println("Problem opening file for writing: " + generateConfigFile)
			fmt.Println(err)
			return
		}
		_, err = newConfigFile.Write(yamlData)
		if err != nil {
			fmt.Println("Error writing to file:", err)
			return
		}
		os.Exit(0)
	}

	if configFile == "" {
		fmt.Println("Config file must be specified with --configfile or -c.")
		os.Exit(1)
	}

	portTypeMap := map[int]string{
		3306:  "App to DB",
		9200:  "App to Elasticsearch",
		5671:  "App to RabbitMQ",
		61613: "App to RabbitMQ",
		61614: "App to RabbitMQ",
		5672:  "App to RabbitMQ",
		4444:  "Percona",
		4567:  "Percona",
		4568:  "Percona",
		4369:  "RabbitMQ",
		25672: "RabbitMQ",
		9300:  "Elasticsearch",
	}

	_, err = os.Stat(configFile)

	var wg1 sync.WaitGroup
	var wgTesting sync.WaitGroup
	var s ServerModel

	if os.IsNotExist(err) {
		fmt.Printf("Config file %v not found\n", configFile)
		os.Exit(1)
	} else {
		data, err := os.ReadFile(configFile)
		if err != nil {
			fmt.Printf("Error reading YAML file: %v", err)
			return
		}

		err = yaml.Unmarshal(data, &s)
		if err != nil {
			fmt.Printf("Error unmarshaling YAML: %v", err)
			return
		}
	}

	currentUser, err := user.Current()
	if err != nil {
		log.Fatalf(err.Error())
	}

	if pass {
		auth = goph.Password(askPass("Enter SSH Password: "))
	} else {
		auth, err = goph.Key(key, getPassphrase(passphrase))
		if err != nil {
			log.Fatal(err)
		}
	}

	conns := make(map[string]*goph.Client)
	results := make(map[int]Result)

	for _, server := range s.Servers {

		wg1.Add(1)
		go func(server Server) {
			defer wg1.Done()
			conns[server.Name], err = makeSshConnection(server.IP, currentUser.Username, &auth)
			if err != nil {
				log.Fatalf("Failed to create ssh connection on %s: %v", server.IP, err)
			}

			output, err := runCommand(conns[server.Name], "mkdir -p ~/.morpheustesting")
			if err != nil {
				log.Fatalf("Error running command on %v", server.Name)
				log.Fatal(err)
				os.Exit(1)
			}
			_ = output

			sftp, err := conns[server.Name].NewSftp()
			if err != nil {
				fmt.Println("Cannot create sftp connection to " + server.Name)
				os.Exit(1)
			}
			defer sftp.Close()

			localSender, err := os.Open("sender")
			if err != nil {
				fmt.Printf("Error opening sender: %v\n", err)
				os.Exit(1)
			}
			defer localSender.Close()

			localReceiver, err := os.Open("receiver")
			if err != nil {
				fmt.Printf("Error opening receiver: %v\n", err)
				os.Exit(1)
			}
			defer localReceiver.Close()

			remoteSender, err := sftp.Create(".morpheustesting/sender")
			if err != nil {
				fmt.Printf("Failed to create remote file on %v: %v\n", server.Name, err)
				os.Exit(1)
			}
			defer remoteSender.Close()

			remoteReceiver, err := sftp.Create(".morpheustesting/receiver")
			if err != nil {
				fmt.Printf("Failed to create remote file: %v\n", err)
				os.Exit(1)
			}
			defer remoteReceiver.Close()

			senderCopy, err := io.Copy(remoteSender, localSender)
			if err != nil {
				fmt.Printf("Error copying file: %v\n", err)
				os.Exit(1)
			}
			_ = senderCopy

			receiverCopy, err := io.Copy(remoteReceiver, localReceiver)
			if err != nil {
				fmt.Printf("Error copying file: %v\n", err)
				os.Exit(1)
			}
			_ = receiverCopy

			output, err = runCommand(conns[server.Name], "chmod +x ~/.morpheustesting/{sender,receiver}")
			if err != nil {
				log.Fatalf("Error running command on %v", server.Name)
				log.Fatal(err)
				os.Exit(1)
			}
			_ = output

		}(server)

	}
	wg1.Wait()
	// Ports to check based on server types

	for _, baseServer := range s.Servers {
		for _, targetServer := range s.Servers {
			if targetServer.Name != baseServer.Name {
				wgTesting.Add(1)
				go func(baseServer, targetServer Server) {
					defer wgTesting.Done()
					// testComms(conns, baseServer, targetServer, getPortsToTest(baseServer, targetServer), portTypeMap, results)
					for _, port := range getPortsToTest(baseServer, targetServer) {
						output, err = testComms(conns, baseServer, targetServer, port)
						if err != nil {
							newResult := Result{
								Source:      baseServer.Name,
								Destination: targetServer.Name,
								Path:        baseServer.IP + " -> " + targetServer.IP + ":" + strconv.Itoa(port),
								Service:     portTypeMap[port],
								Success:     false,
								Error:       "Output: " + output + "Error: " + err.Error() + "\n",
							}
							results[len(results)] = newResult
						} else {
							newResult := Result{
								Source:      baseServer.Name,
								Destination: targetServer.Name,
								Path:        baseServer.IP + " -> " + targetServer.IP + ":" + strconv.Itoa(port),
								Service:     portTypeMap[port],
								Success:     true,
							}
							results[len(results)] = newResult
						}
					}
				}(baseServer, targetServer)
			}
		}
	}
	wgTesting.Wait()

	var resultsSlice []Result
	for _, result := range results {
		resultsSlice = append(resultsSlice, result)
	}

	// Sort the slice based on the Source field
	sort.Slice(resultsSlice, func(i, j int) bool {
		return resultsSlice[i].Source < resultsSlice[j].Source
	})

	// Print the sorted results
	for _, result := range resultsSlice {
		printOutput(result)

	}

}

func printOutput(result Result) {
	errorRed := color.New(color.FgRed).SprintFunc()
	fmt.Printf("Source: %s ", result.Source)
	fmt.Printf("Destination: %s ", result.Destination)
	fmt.Printf("Path: %s ", result.Path)
	fmt.Printf("Service: %s ", result.Service)
	if result.Success {
		fmt.Printf("Success: true\n")
	} else {
		fmt.Print(errorRed("Success: false\n", result.Error))
	}
}

func getPortsToTest(source, destination Server) []int {
	perconaIntra := []int{4444, 4567, 4568}
	rabbitmqIntra := []int{4369, 25672}
	elasticsearchIntra := []int{9300}
	appToDb := []int{3306}
	appToRabbit := []int{5672, 5671, 61613, 61614}
	appToElasticsearch := []int{9200}
	if source.PerconaNode && destination.PerconaNode {
		return perconaIntra
	} else if source.RabbitNode && destination.RabbitNode {
		return rabbitmqIntra
	} else if source.ElasticNode && destination.ElasticNode {
		return elasticsearchIntra
	} else if source.AppNode {
		if destination.PerconaNode {
			return appToDb
		} else if destination.RabbitNode {
			return appToRabbit
		} else if destination.ElasticNode {
			return appToElasticsearch
		}
	}
	return []int{}
}

func makeSshConnection(sshHost string, currentUser string, sshAuth *goph.Auth) (*goph.Client, error) {
	// conn, err := goph.New(currentUser, sshHost, *sshAuth)
	conn, err := goph.NewConn(&goph.Config{
		User:     currentUser,
		Addr:     sshHost,
		Port:     22,
		Auth:     *sshAuth,
		Callback: verifyHost,
	})
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func testComms(conns map[string]*goph.Client, baseServer Server, target Server, port int) (output string, err error) {

	output, err = runCommand(conns[target.Name], "~/.morpheustesting/receiver -p "+strconv.Itoa(port))
	if err != nil {
		return output, err
	}
	// _ = output
	time.Sleep(1 * time.Second)
	output, err = runCommand(conns[baseServer.Name], "~/.morpheustesting/sender -h "+target.IP+" -p "+strconv.Itoa(port))
	return output, err
}

func runCommand(conn *goph.Client, command string) (string, error) {

	output, err := conn.Run(command)
	if err != nil {
		return string(output), err
	}
	return string(output), nil
}

func verifyHost(host string, remote net.Addr, key ssh.PublicKey) error {

	hostFound, err := goph.CheckKnownHost(host, remote, key, "")

	// Host in known hosts but key mismatch!
	// Maybe because of MAN IN THE MIDDLE ATTACK!
	if hostFound && err != nil {

		return err
	}

	// handshake because public key already exists.
	if hostFound && err == nil {

		return nil
	}

	// Ask user to check if he trust the host public key.
	// if !askIsHostTrusted(host, key) {
	if !accepthostkeys {

		// Make sure to return error on non trusted keys.
		return errors.New("some host keys are missing from known_hosts, use -accepthostkeys to accept them all")
	}

	// Add the new host to known hosts file.
	return goph.AddKnownHost(host, remote, key, "")
}

func askPass(msg string) string {

	fmt.Print(msg)
	pass, err := term.ReadPassword(0)
	if err != nil {
		panic(err)
	}

	fmt.Println("")
	return strings.TrimSpace(string(pass))
}

func getPassphrase(ask bool) string {
	if ask {
		return askPass("Enter Private Key Passphrase: ")
	}
	return ""
}
