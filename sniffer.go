package main

import (
	"fmt"
	"os"
	"time"

	"github.com/gizak/termui/v3"
)

func exit(s string) {
	fmt.Println("Start sniffer failed:", s)
	os.Exit(1)
}

// Options is the options set for the sniffer instance.
type Options struct {
	// BPFFilter is the string pcap filter with the BPF syntax
	// eg. "tcp and port 80"
	BPFFilter string

	// Interval is the interval for refresh rate in seconds
	Interval int

	// ViewMode represents the sniffer view mode, optional: bytes, packets, processes
	ViewMode ViewMode

	// DevicesPrefix represents prefixed devices to monitor
	DevicesPrefix []string

	// Pids to watch in processes mode
	Pids []int

	// Unit of stats in processes mode, optional: B, KB, MB, GB
	Unit Unit

	// DisableDNSResolve decides whether if disable the DNS resolution
	DisableDNSResolve bool
}

func (o Options) Validate() error {
	if err := o.ViewMode.Validate(); err != nil {
		return err
	}
	if err := o.Unit.Validate(); err != nil {
		return err
	}
	return nil
}

func DefaultOptions() Options {
	return Options{
		BPFFilter:         "tcp or udp",
		Interval:          1,
		ViewMode:          ModeTableBytes,
		Unit:              UnitKB,
		DisableDNSResolve: false,
	}
}

type Sniffer struct {
	opts          Options
	dnsResolver   *DNSResolver
	pcapClient    *PcapClient
	statsManager  *StatsManager
	ui            *UIComponent
	socketFetcher SocketFetcher
}

func NewSniffer(opts Options) (*Sniffer, error) {
	dnsResolver := NewDnsResolver()
	pcapClient, err := NewPcapClient(dnsResolver.Lookup, opts)
	if err != nil {
		return nil, err
	}

	return &Sniffer{
		opts:          opts,
		dnsResolver:   dnsResolver,
		pcapClient:    pcapClient,
		statsManager:  NewStatsManager(opts),
		ui:            NewUIComponent(opts),
		socketFetcher: GetSocketFetcher(),
	}, nil
}

func (s *Sniffer) Start() {
	events := termui.PollEvents()
	s.Refresh()
	var paused bool

	ticker := time.Tick(time.Duration(s.opts.Interval) * time.Second)
	for {
		select {
		case e := <-events:
			switch e.ID {
			case "<Tab>":
				s.ui.viewer.Shift()
			case "<Space>":
				paused = !paused
			case "<Resize>":
				payload := e.Payload.(termui.Resize)
				s.ui.viewer.Resize(payload.Width, payload.Height)
			case "q", "Q", "<C-c>":
				return
			}

		case <-ticker:
			if !paused {
				s.Refresh()
			}
		}
	}
}

func (s *Sniffer) Close() {
	s.pcapClient.Close()
	s.dnsResolver.Close()
	s.ui.Close()
}

func (s *Sniffer) Refresh() {
	utilization := s.pcapClient.GetUtilization()
	openSockets, err := s.socketFetcher.GetOpenSockets()
	if err != nil {
		return
	}

	s.statsManager.Put(Stat{OpenSockets: openSockets, Utilization: utilization})
	s.ui.viewer.Render(s.statsManager.GetStats())
}
