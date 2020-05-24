package main

import (
	"encoding/json"
	"errors"
	"log"
	"net"
	"runtime"
	"time"
)

type SearchAndShareFile struct {
	ReceivePacket chan SearchPacket
	stop          chan int
	timer         *time.Timer
	ip            net.IP
	broadcast_ip  *net.UDPAddr
	conn          net.PacketConn
	code          map[string]bool
	searched_code map[string]bool
	pause         map[string]bool
	sharefile     *ShareFile
	mask          net.IPMask
}

func CreateInterfaceList(ip net.IP, mask net.IPMask) (*SearchAndShareFile, error) {
	var SearchObj *SearchAndShareFile = new(SearchAndShareFile)

	ip = ip.To4()
	log.Printf("listen on %s", ip)
	SearchObj.ip = ip
	SearchObj.ReceivePacket = make(chan SearchPacket)
	SearchObj.code = make(map[string]bool)
	SearchObj.pause = make(map[string]bool)
	SearchObj.searched_code = make(map[string]bool)
	SearchObj.mask = mask

	var err error
	broadcast_ipaddr, err := SearchObj.ToBroadcastAddress(ip)
	if err != nil {
		return nil, err
	}
	broadcast_udp_addr, err := net.ResolveUDPAddr(
		"udp4", broadcast_ipaddr.String()+":9982")
	SearchObj.broadcast_ip = broadcast_udp_addr
	if err != nil {
		return nil, err
	}

	// create connection
	if runtime.GOOS == "windows" {
		SearchObj.conn, err = net.ListenPacket("udp4", ip.String()+":9982")
	} else {
		SearchObj.conn, err = net.ListenPacket("udp4", broadcast_udp_addr.String())
	}
	if err != nil {
		return nil, err
	}

	go func() {
		buf := make([]byte, 8192)
		log.Println("Start Listening")
		for n, addr, err := SearchObj.conn.ReadFrom(buf); err == nil; n, addr, err = SearchObj.conn.ReadFrom(buf) {
			var packet SearchPacket
			err = json.Unmarshal(buf[:n], &packet)
			if err != nil {
				log.Println("get udp packet error ", err)
			} else {
				err = DecodePacket(&packet)
				if err != nil {
					log.Println("decode udp packet error ", err)
				}
				packet.IP = addr.(*net.UDPAddr)
				if packet.IP.IP.String() != SearchObj.ip.String() {
					log.Printf("receive %+v from %s\n", packet, addr)
					switch packet.Data.(type) {
					case *FileRequest:
						SearchObj.sendResponse(packet)
					case *FileResponse:
						response := packet.Data.(*FileResponse)
						_, ok := SearchObj.code[response.Code]
						if ok {
							SearchObj.ReceivePacket <- packet
							SearchObj.searched_code[response.Code] = true
						}
					}
				}
			}
		}
	}()

	//listen
	return SearchObj, nil
}

func (self SearchAndShareFile) ToBroadcastAddress(ip net.IP) (net.IP, error) {
	broadcast_ipaddr := ip.To4()
	if broadcast_ipaddr == nil {
		return nil, errors.New("ip is nil")
	}
	broadcast_ipaddr = broadcast_ipaddr.Mask(self.mask)
	mask := ip.DefaultMask()
	for i := range self.mask {
		mask[i] = (255 ^ self.mask[i])
	}
	if mask[0] != 0 {
		return nil, errors.New("not found")
	}
	for i := range broadcast_ipaddr {
		broadcast_ipaddr[i] = mask[i] | broadcast_ipaddr[i]
	}
	return broadcast_ipaddr, nil
}

func (self SearchAndShareFile) broadcast() {
	log.Printf("broadcast to %s\n", self.broadcast_ip)
	code := make([]string, 0, 5)
	for val := range self.code {
		if !self.pause[val] && !self.searched_code[val] {
			code = append(code, val)
		}
	}
	self.searched_code = make(map[string]bool)
	if len(code) == 0 {
		return
	}
	data, err := EncodeRequestPacket(&FileRequest{
		Code: code,
	})
	if err != nil {
		log.Printf("broadcast failed, error %s", err)
	}
	_, err = self.conn.WriteTo(data, self.broadcast_ip)
	if err != nil {
		log.Printf("broadcast failed, error %s", err)
	}
}

func (self *SearchAndShareFile) SearchFile(code string) error {
	code_tmp := make([]string, len(self.code))
	if self.code[code] {
		return errors.New("code exists")
	}
	for val := range self.code {
		code_tmp = append(code_tmp, val)
	}
	data, err := EncodeRequestPacket(&FileRequest{Code: code_tmp})
	if err != nil {
		return err
	}
	if len(data) > 8192 {
		return errors.New("data too long")
	}
	self.code[code] = true
	return nil
}

func (self *SearchAndShareFile) sendResponse(pkg SearchPacket) {
	request := pkg.Data.(*FileRequest)
	for _, val := range request.Code {
		if file := self.sharefile.code[val]; file != nil && !self.sharefile.working_map[val] {
			response := FileResponse{
				Code:     val,
				Secret:   self.sharefile.Secret,
				Port:     *self.sharefile.Port,
				Path:     "download",
				FileName: file.Name(),
			}
			data, err := EncodeResponsePacket(&response)
			if err != nil {
				log.Printf("encode response error %s", err)
				return
			}

			ip, err := self.ToBroadcastAddress(pkg.IP.IP)
			if err != nil {
				log.Printf("ip error %s", err)
				return
			}
			pkg.IP.IP = ip
			log.Printf("send response to %s\n", pkg.IP)
			self.conn.WriteTo(data, pkg.IP)
		}
	}
}

func (self *SearchAndShareFile) Start(broadcast_time time.Duration) {
	if self.stop != nil {
		return
	}
	self.stop = make(chan int)
	self.timer = time.NewTimer(broadcast_time)

	self.broadcast()
	go func() {
	SearchFile:
		for {
			select {
			case <-self.timer.C:
				self.timer.Reset(broadcast_time)
				self.broadcast()
			case val := <-self.stop:
				if val == 0 {
					self.timer.Stop()
				} else if val == 1 {
					self.timer.Reset(broadcast_time)
				} else {
					self.timer.Stop()
					self.conn.Close()
				}
				break SearchFile
			}
		}
	}()
}

func (self *SearchAndShareFile) Stop() {
	self.stop <- 0
}

func (self *SearchAndShareFile) Close() {
	self.stop <- 2
}
