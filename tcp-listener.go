/*
 version go1.15.5 linux/amd64 on Ubuntu 20.04
 compile static build
 CGO_ENABLED=0 GOOS=linux go build -a  -o tcp-listener tcp-listener.go

 when giving large number of ips and ports, it may complains
 "accept4: too many open files...", increase the ulimit number,
 manually remove the ip address and try again
 #ulimit -n 1000000
 #for i in $(seq start, end); do ip a del 10.169.72.$i dev <interface>; done

*/

package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
)

var mask string = "24"

func Hosts(cidr string) ([]string, int, error) {
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, 0, err
	}

	var ips []string
	for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); inc(ip) {
		ips = append(ips, ip.String())
	}

	// remove network address and broadcast address
	lenIPs := len(ips)
	switch {
	case lenIPs < 2:
		return ips, lenIPs, nil

	default:
		return ips[1 : len(ips)-1], lenIPs - 2, nil
	}
}

func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

func hello(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "hello world")
}

func execCmd(command string, argsv []string) (err error) {
	args := argsv
	cmdObj := exec.Command(command, args...)
	cmdObj.Stdout = os.Stdout
	cmdObj.Stderr = os.Stderr
	err = cmdObj.Run()
	if err != nil {
		log.Fatal(err)
		return err
	}
	return nil
}

func SetupCloseHandler(cidr string, device string) {
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Println("\r- Ctrl+C pressed in Terminal")
		RemoveIps(cidr, device)
		os.Exit(0)
	}()
}

func RemoveIps(cidr string, device string) {
	fmt.Println("- Run Clean Up - Remove IPs")

	hosts, _, err := Hosts(cidr)
	if err != nil {
		log.Fatal(err)
	}

	for _, h := range hosts {
		ipaddr := fmt.Sprintf("%s/%s", h, mask)
		execCmd("ip", []string{"addr", "del", ipaddr, "dev", device})
	}

	fmt.Println("- Good bye!")
}

var Usage = func() {
	fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
	flag.PrintDefaults()
}

func blockForever() {
	select {}
}

func main() {

	cidr := flag.String("cidr", "127.1.0.0/24", "ip cidr to listen")
	device := flag.String("device", "lo", "network interface to listen on")
	port := flag.String("port", "55025:55030", "ports to listen")
	flag.Parse()

	ports := strings.Split(*port, ":")

	start, err := strconv.Atoi(ports[0])
	if err != nil {
		// handle error
		fmt.Println(err)
		os.Exit(2)
	}

	end, err := strconv.Atoi(ports[1])
	if err != nil {
		// handle error
		fmt.Println(err)
		os.Exit(2)
	}

	SetupCloseHandler(*cidr, *device)

	serverMux := http.NewServeMux()
	serverMux.HandleFunc("/", hello)

	hosts, _, err := Hosts(*cidr)
	if err != nil {
		log.Fatal(err)
	}

	for _, h := range hosts {
		ipaddr := fmt.Sprintf("%s/%s", h, mask)
		execCmd("ip", []string{"addr", "add", ipaddr, "dev", *device})
	}

	for i := start; i <= end; i++ {
		fmt.Println(i)
		port := i
		for _, host := range hosts {
			host_port := fmt.Sprintf("%s:%d", host, port)
			go func() {
				http.ListenAndServe(host_port, serverMux)
			}()
		}
	}

	blockForever()

}
