package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

type SearchManager struct {
	searchfile    []*SearchAndShareFile
	sharedfile    *ShareFile
	PacketChannel chan SearchPacket
	mutex         sync.Mutex
}

func CreateSearchManager() (*SearchManager, error) {
	var manager SearchManager
	var err error
	manager.searchfile, err = SearchItem()
	if err != nil {
		return nil, err
	}
	if len(manager.searchfile) == 0 {
		return nil, errors.New("can't bind any interface")
	}

	manager.sharedfile = CreateShareObject()
	manager.PacketChannel = make(chan SearchPacket)
	for _, val := range manager.searchfile {
		val.sharefile = manager.sharedfile
		channel := val.ReceivePacket
		go func() {
			for {
				manager.PacketChannel <- <-channel
			}
		}()
	}

	return &manager, err
}

func (self *SearchManager) pause(code string) bool {
	self.mutex.Lock()
	defer self.mutex.Unlock()
	if self.searchfile[0].pause[code] {
		return false
	}
	for _, val := range self.searchfile {
		val.pause[code] = true
	}
	return true
}

func (self *SearchManager) resume(code string) {
	for i := len(self.searchfile) - 1; i > -1; i-- {
		delete(self.searchfile[i].pause, code)
	}
}

func (self *SearchManager) RemoveSearch(code string) {
	for _, val := range self.searchfile {
		delete(val.code, code)
	}
}

func (self *SearchManager) ShareFile(code string, file *os.File) error {
	return self.sharedfile.Share(code, file)
}

func (self *SearchManager) DownloadFile(pkg *FileResponse, host, path string, callback func(delta int, max_size int)) (*os.File, error) {
	tr := &http.Transport{
		IdleConnTimeout: 10 * time.Second,
	}
	self.pause(pkg.Code)
	defer self.resume(pkg.Code)

	client := &http.Client{Transport: tr}
	request, err := client.Get(fmt.Sprintf(
		"http://%s:%d/%s?secret=%s&code=%s",
		host,
		pkg.Port,
		pkg.Path,
		pkg.Secret,
		pkg.Code,
	))
	if err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		file.Close()
		return nil, err
	}
	if request.StatusCode != http.StatusOK {
		file.Close()
		return nil, errors.New(fmt.Sprintf(
			"Return Status is %d",
			request.StatusCode,
		))
	}
	buf := make([]byte, 10240)
	log.Println("download file size is ", request.Header.Get("Content-Length"))
	max_size, err := strconv.Atoi(request.Header.Get("Content-Length"))
	if err != nil {
		return nil, errors.New("Content-Length is not a number")
	}

	for n, err := request.Body.Read(buf); ; n, err = request.Body.Read(buf) {
		if err != io.EOF && err != nil {
			fmt.Println(err)
			file.Close()
			os.Remove(path)
			return nil, err
		}
		file.Write(buf[:n])
		callback(n, max_size)
		if err == io.EOF {
			break
		}
	}

	if err != nil && err != io.EOF {
		file.Close()
		os.Remove(path)
		return nil, err
	}
	file.Close()
	file, err = os.OpenFile(path, os.O_RDONLY, 0644)
	if err != nil {
		return nil, err
	}
	self.RemoveSearch(pkg.Code)
	return file, nil
}

func (self *SearchManager) SearchStart(broadcast_time time.Duration) {
	for _, val := range self.searchfile {
		val.Start(broadcast_time)
	}
}

func (self *SearchManager) SearchFile(code string) error {
	for _, val := range self.searchfile {
		err := val.SearchFile(code)
		if err != nil {
			return err
		}
	}
	return nil
}

func (self *SearchManager) ShareStart() {
	self.sharedfile.Start()
}

func (self *SearchManager) SearchStop() {
	for _, val := range self.searchfile {
		val.Stop()
	}
}
func (self *SearchManager) Close() {
	for _, val := range self.searchfile {
		val.Close()
	}
}

func SearchItem() ([]*SearchAndShareFile, error) {
	ifaces, err := net.Interfaces()
	result := make([]*SearchAndShareFile, 0, 5)
	if err != nil {
		return nil, err
	}
	for _, i := range ifaces {
		ipset, err := i.Addrs()
		if err != nil {
			return nil, err
		}
		for _, addr := range ipset {
			var ip net.IP
			var mask net.IPMask
			switch v := addr.(type) {
			case *net.IPNet:
				mask = v.Mask
				ip = v.IP
			default:
				continue
			}
			ip = ip.To4()
			if ip == nil {
				continue
			}
			if ip.IsLoopback() {
				continue
			} else {
				s, err := CreateInterfaceList(ip, mask)
				if err != nil {
					log.Printf("can't bind address on %s, error: %s", ip, err)
					continue
				}
				result = append(result, s)
			}
		}
	}
	return result, nil
}
