package main

import (
	"encoding/json"
	"errors"
	"net"
)

type SearchPacket struct {
	IP   *net.UDPAddr `json:"ip,omitempty"`
	Type string       `json:"type"`
	Data interface{}  `json:"data"`
}

type FileRequest struct {
	Code []string `json:"code""`
}

type FileResponse struct {
	Code     string `json:"code"`
	Secret   string `json:"secret"`
	Port     int    `json:"port"`
	Path     string `json:"path"`
	FileName string `json:"file_name"`
	FileHash string `json:"file_hash"`
}

func DecodePacket(packet *SearchPacket) error {
	data, ok := packet.Data.(map[string]interface{})
	if !ok {
		return errors.New("data type can't be decoded")
	}
	var actully interface{}
	switch packet.Type {
	case "request":
		actully = &FileRequest{}
	case "response":
		actully = &FileResponse{}
	}
	bytes, err := json.Marshal(data)
	err = json.Unmarshal(bytes, actully)
	if err != nil {
		return err
	}
	packet.Data = actully
	return nil
}

func EncodeRequestPacket(packet *FileRequest) ([]byte, error) {
	var request SearchPacket
	request.Type = "request"
	request.Data = packet
	data, err := json.Marshal(request)
	return data, err
}

func EncodeResponsePacket(packet *FileResponse) ([]byte, error) {
	var response SearchPacket
	response.Type = "response"
	response.Data = packet
	data, err := json.Marshal(response)
	return data, err
}

func CreateFileRequest(request FileRequest) ([]byte, error) {
	return json.Marshal(SearchPacket{
		Type: "request",
		Data: request,
	})
}
