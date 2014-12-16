package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"time"
)

const UDP_PACKET_SIZE uint = 65507

type PartHeader struct {
	PartType   uint16
	PartLength uint16
}

type StringPart struct {
	Header PartHeader
	Value  string
}

type NumericPart struct {
	Header PartHeader
	Value  int64
}

type ValuePart struct {
	Header         PartHeader
	NumberOfValues uint16
	Values         []Value
}

type Value struct {
	DataType      byte
	CounterValue  uint64
	GaugeValue    float64
	DeriveValue   int64
	AbsoluteValue int64
}

type Packet struct {
	Host           StringPart
	Time           NumericPart
	TimeHigh       NumericPart
	Plugin         StringPart
	PluginInstance StringPart
	Type           StringPart
	TypeInstance   StringPart
	Values         ValuePart
	Interval       NumericPart
	IntervalValue  NumericPart
	Message        StringPart
	Severity       NumericPart
}

type part func(packet *Packet, payload *bytes.Buffer) (err error)

func plength(length uint16) uint16 {
	return length + 4
}

func PartHeaderFromBuffer(partType uint16, payload *bytes.Buffer) PartHeader {
	return PartHeader{partType, plength(uint16(payload.Len()))}
}

func hostname(packet *Packet, payload *bytes.Buffer) (err error) {
	stringPart := StringPart{PartHeaderFromBuffer(0x0000, payload), payload.String()}
	packet.Host = stringPart
	log.Printf("type = %d, length = %d, hostname = %s",
		packet.Host.Header.PartType,
		packet.Host.Header.PartLength,
		packet.Host.Value)
	return nil
}

func lowtime(packet *Packet, payload *bytes.Buffer) (err error) {
	var value int64
	readErr := binary.Read(payload, binary.BigEndian, &value)
	if readErr != nil {
		return readErr
	} else {
		numericPart := NumericPart{PartHeaderFromBuffer(0x0001, payload), value}
		packet.Time = numericPart
		log.Printf("type = %d, length = %d, hostname = %s",
			packet.Time.Header.PartType,
			packet.Time.Header.PartLength,
			packet.Time.Value)
		return nil
	}
}

func hightime(packet *Packet, payload *bytes.Buffer) (err error) {
	var value int64
	readErr := binary.Read(payload, binary.BigEndian, &value)
	if readErr != nil {
		return readErr
	} else {
		numericPart := NumericPart{PartHeaderFromBuffer(0x0008, payload), value}
		packet.TimeHigh = numericPart
		log.Printf("type = %d, length = %d, datevalue = %s",
			packet.TimeHigh.Header.PartType,
			packet.TimeHigh.Header.PartLength,
			time.Unix(packet.TimeHigh.Value>>30, 0))
		return nil
	}
}

func plugin(packet *Packet, payload *bytes.Buffer) (err error) {
	stringPart := StringPart{PartHeaderFromBuffer(0x0002, payload), payload.String()}
	packet.Plugin = stringPart
	log.Printf("type = %d, length = %d, plugin-name = %s",
		packet.Plugin.Header.PartType,
		packet.Plugin.Header.PartLength,
		packet.Plugin.Value)
	return nil
}

func pluginInstance(packet *Packet, payload *bytes.Buffer) (err error) {
	stringPart := StringPart{PartHeaderFromBuffer(0x0003, payload), payload.String()}
	packet.PluginInstance = stringPart
	log.Printf("type = %d, length = %d, plugin-instance = %s",
		packet.PluginInstance.Header.PartType,
		packet.PluginInstance.Header.PartLength,
		packet.PluginInstance.Value)
	return nil
}

func processType(packet *Packet, payload *bytes.Buffer) (err error) {
	stringPart := StringPart{PartHeaderFromBuffer(0x0004, payload), payload.String()}
	packet.Type = stringPart
	log.Printf("type = %d, length = %d, type = %s",
		packet.Type.Header.PartType,
		packet.Type.Header.PartLength,
		packet.Type.Value)
	return nil
}

func processTypeInstance(packet *Packet, payload *bytes.Buffer) (err error) {
	stringPart := StringPart{PartHeaderFromBuffer(0x0005, payload), payload.String()}
	packet.TypeInstance = stringPart
	log.Printf("type = %d, length = %d, type-instance = %s",
		packet.TypeInstance.Header.PartType,
		packet.TypeInstance.Header.PartLength,
		packet.TypeInstance.Value)
	return nil
}

func interval(packet *Packet, payload *bytes.Buffer) (err error) {
	var value int64
	readErr := binary.Read(payload, binary.BigEndian, &value)
	if readErr != nil {
		return readErr
	} else {
		numericPart := NumericPart{PartHeaderFromBuffer(0x0008, payload), value}
		packet.Interval = numericPart
		log.Printf("type = %d, length = %d, datevalue = %s",
			packet.Interval.Header.PartType,
			packet.Interval.Header.PartLength,
			time.Unix(packet.Interval.Value, 0))
		return nil
	}
}

func createMessageProcessors() (processors map[uint16]part) {

	//Need to look at returning a touple here being the id the func is designed to work with
	//and the actual func itself.  This could then be simplified into an array

	messageProcessors := make(map[uint16]part)
	messageProcessors[0x0000] = hostname
	messageProcessors[0x0001] = lowtime
	messageProcessors[0x0008] = hightime
	messageProcessors[0x0002] = plugin
	messageProcessors[0x0003] = pluginInstance
	messageProcessors[0x0004] = processType
	messageProcessors[0x0005] = processTypeInstance
	return messageProcessors
}

func main() {
	messageProcessors := createMessageProcessors()

	uaddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", 5555))
	if err != nil {
		log.Fatal(err)
	}
	conn, err := net.ListenUDP("udp", uaddr)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	packet := new(Packet)
	packetBytes := make([]byte, UDP_PACKET_SIZE)

	for {
		numOfBytesReceived, _, err := conn.ReadFromUDP(packetBytes)
		packetBytes = packetBytes[0:numOfBytesReceived]

		if err != nil {
			log.Fatal(err)
		}
		buffer := bytes.NewBuffer(packetBytes)
		go func(payloadBuffer *bytes.Buffer) {
			for payloadBuffer.Len() > 0 {
				partHeader := new(PartHeader)
				binary.Read(payloadBuffer, binary.BigEndian, partHeader)
				partBuffer := bytes.NewBuffer(payloadBuffer.Next(int(partHeader.PartLength) - 4))
				processor, supports := messageProcessors[partHeader.PartType]
				if supports {
					err := processor(packet, partBuffer)
					if err != nil {
						log.Fatal(err)
					}
				} else {
					fmt.Print(".")
				}
			}
			fmt.Print("\n")
		}(buffer)
	}
}