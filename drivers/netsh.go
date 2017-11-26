package drivers

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"

	"github.com/Sirupsen/logrus"
)

const (
	NetshDriverName = "Netsh"
	NatAdapterName  = "NatAdapter"
	seperator       = "---------------------------\n"
)

type Netsh struct {
	adapterName string
}

func (driver *Netsh) Init(config map[string]interface{}) error {

	var err error
	adapterName, ok := config[NatAdapterName]
	if !ok {
		return errors.New("netsh driver config missing: NatAdapterName")
	}
	i, err := net.InterfaceByName(adapterName.(string))
	if err != nil {
		return err
	}
	logrus.Debugf("interface is :%#v", i)
	driver.adapterName = `"` + adapterName.(string) + `"`
	//uninstall
	cmd := exec.Command("netsh", "routing", "ip", "nat", "uninstall")
	if err = cmd.Run(); err != nil {
		return err
	}
	//install
	cmd = exec.Command("netsh", "routing", "ip", "nat", "install")
	if err = cmd.Run(); err != nil {
		return err
	}
	//add interface to nat driver
	cmd = exec.Command("netsh", "routing", "ip", "nat", "add", "interface", driver.adapterName, "full")
	if err = cmd.Run(); err != nil {
		return err
	}
	return nil
}
func (driver *Netsh) CreatePortMapping(
	externalIP net.IP,
	externalPort uint32,
	internalIP net.IP,
	internalPort uint32,
	Protocol string) (PortMapping, error) {

	rtn := PortMapping{
		ExternalIP:   externalIP,
		ExternalPort: externalPort,
		InternalIP:   internalIP,
		InternalPort: internalPort,
		Protocol:     Protocol,
	}
	errbuff := bytes.NewBuffer([]byte{})
	outbuff := bytes.NewBuffer([]byte{})
	cmd := exec.Command(
		"netsh",
		"routing",
		"ip",
		"nat",
		"add",
		"portmapping",
		driver.adapterName,
		rtn.Protocol,
		rtn.ExternalIP.String(),
		strconv.FormatUint(uint64(rtn.ExternalPort), 10),
		rtn.InternalIP.String(),
		strconv.FormatUint(uint64(rtn.InternalPort), 10),
	)
	cmd.Stderr = errbuff
	cmd.Stdout = outbuff
	err := cmd.Run()
	if err != nil {
		println(outbuff.String())
		println(errbuff.String())
	}
	return rtn, err
}
func (driver *Netsh) ListPortMapping() ([]PortMapping, error) {
	outputBuffer := bytes.NewBuffer([]byte{})
	cmd := exec.Command("netsh", "routing", "ip", "nat", "show", "interface", driver.adapterName)
	cmd.Stdout = outputBuffer
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	output := strings.Replace(outputBuffer.String(), "\r", "", -1)
	blocks := strings.Split(output, seperator)
	switch len(blocks) {
	case 1: //when only one element in the array, it doesn't have any nat interface in RRAS service
		return nil, fmt.Errorf("%s is not a nat interface", driver.adapterName)
	case 2: //when there are two element in the array, there is nothing in the PortMapping list
		return []PortMapping{}, nil
	case 3: //there are some port mapping rules in this interface
	default:
		logrus.Debug(output)
		return nil, errors.New("driver Netsh error, ListPortMapping get unexpected output")
	}

	//one portmapping object will be like following
	/*
		protocol    : TCP
		publicip    : 0.0.0.0
		publicport  : 80
		privateip   : 192.169.1.100
		privateport : 80
	*/
	scanner := bufio.NewScanner(strings.NewReader(blocks[2]))
	current := &PortMapping{}
	currentLineCount := 0
	rtn := []PortMapping{}
	//scanning portmapping objects to struct
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 {
			rtn = append(rtn, *current)
			current = &PortMapping{}
			currentLineCount = 0
			continue
		}
		value := strings.TrimSpace(strings.Split(line, ":")[1])
		switch currentLineCount {
		case 0:
			current.Protocol = value
		case 1:
			current.ExternalIP = net.ParseIP(value)
		case 2:
			port, _ := strconv.Atoi(value)
			current.ExternalPort = uint32(port)
		case 3:
			current.InternalIP = net.ParseIP(value)
		case 4:
			port, _ := strconv.Atoi(value)
			current.InternalPort = uint32(port)
		}
		currentLineCount++
	}
	return rtn, nil
}
func (driver *Netsh) DeletePortMapping(tar PortMapping) error {
	return exec.Command(
		"netsh",
		"routing",
		"ip",
		"nat",
		"delete",
		"portmapping",
		driver.adapterName,
		tar.Protocol,
		tar.ExternalIP.String(),
		strconv.FormatUint(uint64(tar.ExternalPort), 10),
	).Run()
}
func (driver *Netsh) Destory() error {
	return nil
}
