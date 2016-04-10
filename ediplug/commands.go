package ediplug

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"time"

	"golang.org/x/net/html/charset"
)

type Command interface {
	GetXML() ([]byte, error)
	Parse(in io.Reader) error
}

func ExecuteCommand(c Command, ip string, password string) error {
	bodyRaw, err := c.GetXML()
	if err != nil {
		return err
	}

	body := bytes.NewBuffer(append([]byte(xml.Header), bodyRaw...))
	req, _ := http.NewRequest("POST", fmt.Sprintf("http://%s:10000/smartplug.cgi", ip), body)
	req.Header.Set("Content-Type", "application/xml")
	req.SetBasicAuth("admin", password)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return c.Parse(resp.Body)
}

type SetStateCommand struct {
	DesiredState string
	Success      bool
}

func (s *SetStateCommand) GetXML() ([]byte, error) {
	c := struct {
		XMLName xml.Name `xml:"SMARTPLUG"`
		ID      string   `xml:"id,attr"`
		Command struct {
			ID    string `xml:"id,attr"`
			State string `xml:"Device.System.Power.State"`
		} `xml:"CMD"`
	}{}
	c.ID = "edimax"
	c.Command.ID = "setup"
	c.Command.State = s.DesiredState

	return xml.Marshal(c)
}

func (s *SetStateCommand) Parse(in io.Reader) error {
	c := struct {
		XMLName xml.Name `xml:"SMARTPLUG"`
		ID      string   `xml:"id,attr"`
		Command string   `xml:"CMD"`
	}{}

	decoder := xml.NewDecoder(in)
	decoder.CharsetReader = charset.NewReaderLabel
	if err := decoder.Decode(&c); err != nil {
		return err
	}

	s.Success = c.Command == "OK"
	return nil
}

type GetStateCommand struct {
	CurrentState string

	comm struct {
		XMLName xml.Name `xml:"SMARTPLUG"`
		ID      string   `xml:"id,attr"`
		Command struct {
			ID    string `xml:"id,attr"`
			State string `xml:"Device.System.Power.State"`
		} `xml:"CMD"`
	}
}

func (g *GetStateCommand) GetXML() ([]byte, error) {
	g.comm.ID = "edimax"
	g.comm.Command.ID = "get"

	return xml.Marshal(g.comm)
}

func (g *GetStateCommand) Parse(in io.Reader) error {
	decoder := xml.NewDecoder(in)
	decoder.CharsetReader = charset.NewReaderLabel
	if err := decoder.Decode(&g.comm); err != nil {
		return err
	}

	g.CurrentState = g.comm.Command.State
	return nil
}

type GetEnergyCommand struct {
	LastToggleTime time.Time
	NowCurrent     float64
	NowPower       float64
	DailyEnergy    float64
	WeeklyEnergy   float64
	MonthlyEnergy  float64

	comm struct {
		XMLName xml.Name `xml:"SMARTPLUG"`
		ID      string   `xml:"id,attr"`
		Command struct {
			ID       string `xml:"id,attr"`
			NowPower struct {
				LastToggleTime string  `xml:"Device.System.Power.LastToggleTime,omitempty"`
				NowCurrent     float64 `xml:"Device.System.Power.NowCurrent,omitempty"`
				NowPower       float64 `xml:"Device.System.Power.NowPower,omitempty"`
				DailyEnergy    float64 `xml:"Device.System.Power.NowEnergy.Day,omitempty"`
				WeeklyEnergy   float64 `xml:"Device.System.Power.NowEnergy.Week,omitempty"`
				MonthlyEnergy  float64 `xml:"Device.System.Power.NowEnergy.Month,omitempty"`
			} `xml:"NOW_POWER"`
		} `xml:"CMD"`
	}
}

func (g *GetEnergyCommand) GetXML() ([]byte, error) {
	g.comm.ID = "edimax"
	g.comm.Command.ID = "get"

	return xml.Marshal(g.comm)
}

func (g *GetEnergyCommand) Parse(in io.Reader) error {
	decoder := xml.NewDecoder(in)
	decoder.CharsetReader = charset.NewReaderLabel
	if err := decoder.Decode(&g.comm); err != nil {
		return err
	}

	t, err := time.Parse("20060102150405", g.comm.Command.NowPower.LastToggleTime)
	if err != nil {
		return err
	}

	g.LastToggleTime = t
	g.NowCurrent = g.comm.Command.NowPower.NowCurrent
	g.NowPower = g.comm.Command.NowPower.NowPower
	g.DailyEnergy = g.comm.Command.NowPower.DailyEnergy
	g.WeeklyEnergy = g.comm.Command.NowPower.WeeklyEnergy
	g.MonthlyEnergy = g.comm.Command.NowPower.MonthlyEnergy
	return nil
}

type GetSystemInfoCommand struct {
	Model           string
	FirmwareVersion string
	MacAddress      string
	SystemName      string
	DeviceTime      time.Time

	comm struct {
		XMLName xml.Name `xml:"SMARTPLUG"`
		ID      string   `xml:"id,attr"`
		Command struct {
			ID         string `xml:"id,attr"`
			SystemInfo struct {
				Model           string `xml:"Run.Model,omitempty"`
				FirmwareVersion string `xml:"Run.FW.Version,omitempty"`
				MacAddress      string `xml:"Run.LAN.Client.MAC.Address,omitempty"`
				SystemName      string `xml:"Device.System.Name,omitempty"`
			} `xml:"SYSTEM_INFO"`
			DeviceTime string `xml:"Device.System.Time"`
		} `xml:"CMD"`
	}
}

func (g *GetSystemInfoCommand) GetXML() ([]byte, error) {
	g.comm.ID = "edimax"
	g.comm.Command.ID = "get"

	return xml.Marshal(g.comm)
}

func (g *GetSystemInfoCommand) Parse(in io.Reader) error {
	decoder := xml.NewDecoder(in)
	decoder.CharsetReader = charset.NewReaderLabel
	if err := decoder.Decode(&g.comm); err != nil {
		return err
	}

	t, err := time.Parse("20060102150405", g.comm.Command.DeviceTime)
	if err != nil {
		return err
	}

	g.DeviceTime = t
	g.Model = g.comm.Command.SystemInfo.Model
	g.FirmwareVersion = g.comm.Command.SystemInfo.FirmwareVersion
	g.MacAddress = g.comm.Command.SystemInfo.MacAddress
	g.SystemName = g.comm.Command.SystemInfo.SystemName

	return nil
}
