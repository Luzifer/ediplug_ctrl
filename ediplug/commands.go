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

// Command is an interface wich includes a XML generator and a XML parser
type Command interface {
	GetXML() ([]byte, error)
	Parse(in io.Reader) error
}

// ExecuteCommand sends a command to a plug and parses the answer. The response can be found in the command object
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

// SetStateCommand switches a plug on or off. DesiredState needs to be set to "ON" or "OFF" (in caps)
type SetStateCommand struct {
	DesiredState string
	Success      bool
}

// GetXML assembles the request XML
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

// Parse parses data from the response XML
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

// GetStateCommand retrieves the current state from the plug (CurrentState will be "ON" or "OFF")
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

// GetXML assembles the request XML
func (g *GetStateCommand) GetXML() ([]byte, error) {
	g.comm.ID = "edimax"
	g.comm.Command.ID = "get"

	return xml.Marshal(g.comm)
}

// Parse parses data from the response XML
func (g *GetStateCommand) Parse(in io.Reader) error {
	decoder := xml.NewDecoder(in)
	decoder.CharsetReader = charset.NewReaderLabel
	if err := decoder.Decode(&g.comm); err != nil {
		return err
	}

	g.CurrentState = g.comm.Command.State
	return nil
}

// GetEnergyCommand retrieves the energy stats from the plug
type GetEnergyCommand struct {
	LastToggleTime time.Time
	NowCurrent     float64 // Measured in Ampere
	NowPower       float64 // Measured in Watts
	DailyEnergy    float64 // Measured in kWh
	WeeklyEnergy   float64 // Measured in kWh
	MonthlyEnergy  float64 // Measured in kWh

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

// GetXML assembles the request XML
func (g *GetEnergyCommand) GetXML() ([]byte, error) {
	g.comm.ID = "edimax"
	g.comm.Command.ID = "get"

	return xml.Marshal(g.comm)
}

// Parse parses data from the response XML
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

// GetSystemInfoCommand retrieves general information about the plug
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

// GetXML assembles the request XML
func (g *GetSystemInfoCommand) GetXML() ([]byte, error) {
	g.comm.ID = "edimax"
	g.comm.Command.ID = "get"

	return xml.Marshal(g.comm)
}

// Parse parses data from the response XML
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
