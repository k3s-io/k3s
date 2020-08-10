// +build linux

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/intelrdt"
	"github.com/opencontainers/runc/types"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var eventsCommand = cli.Command{
	Name:  "events",
	Usage: "display container events such as OOM notifications, cpu, memory, and IO usage statistics",
	ArgsUsage: `<container-id>

Where "<container-id>" is the name for the instance of the container.`,
	Description: `The events command displays information about the container. By default the
information is displayed once every 5 seconds.`,
	Flags: []cli.Flag{
		cli.DurationFlag{Name: "interval", Value: 5 * time.Second, Usage: "set the stats collection interval"},
		cli.BoolFlag{Name: "stats", Usage: "display the container's stats then exit"},
	},
	Action: func(context *cli.Context) error {
		if err := checkArgs(context, 1, exactArgs); err != nil {
			return err
		}
		container, err := getContainer(context)
		if err != nil {
			return err
		}
		duration := context.Duration("interval")
		if duration <= 0 {
			return errors.New("duration interval must be greater than 0")
		}
		status, err := container.Status()
		if err != nil {
			return err
		}
		if status == libcontainer.Stopped {
			return fmt.Errorf("container with id %s is not running", container.ID())
		}
		var (
			stats  = make(chan *libcontainer.Stats, 1)
			events = make(chan *types.Event, 1024)
			group  = &sync.WaitGroup{}
		)
		group.Add(1)
		go func() {
			defer group.Done()
			enc := json.NewEncoder(os.Stdout)
			for e := range events {
				if err := enc.Encode(e); err != nil {
					logrus.Error(err)
				}
			}
		}()
		if context.Bool("stats") {
			s, err := container.Stats()
			if err != nil {
				return err
			}
			events <- &types.Event{Type: "stats", ID: container.ID(), Data: convertLibcontainerStats(s)}
			close(events)
			group.Wait()
			return nil
		}
		go func() {
			for range time.Tick(context.Duration("interval")) {
				s, err := container.Stats()
				if err != nil {
					logrus.Error(err)
					continue
				}
				stats <- s
			}
		}()
		n, err := container.NotifyOOM()
		if err != nil {
			return err
		}
		for {
			select {
			case _, ok := <-n:
				if ok {
					// this means an oom event was received, if it is !ok then
					// the channel was closed because the container stopped and
					// the cgroups no longer exist.
					events <- &types.Event{Type: "oom", ID: container.ID()}
				} else {
					n = nil
				}
			case s := <-stats:
				events <- &types.Event{Type: "stats", ID: container.ID(), Data: convertLibcontainerStats(s)}
			}
			if n == nil {
				close(events)
				break
			}
		}
		group.Wait()
		return nil
	},
}

func convertLibcontainerStats(ls *libcontainer.Stats) *types.Stats {
	cg := ls.CgroupStats
	if cg == nil {
		return nil
	}
	var s types.Stats
	s.Pids.Current = cg.PidsStats.Current
	s.Pids.Limit = cg.PidsStats.Limit

	s.CPU.Usage.Kernel = cg.CpuStats.CpuUsage.UsageInKernelmode
	s.CPU.Usage.User = cg.CpuStats.CpuUsage.UsageInUsermode
	s.CPU.Usage.Total = cg.CpuStats.CpuUsage.TotalUsage
	s.CPU.Usage.Percpu = cg.CpuStats.CpuUsage.PercpuUsage
	s.CPU.Usage.PercpuKernel = cg.CpuStats.CpuUsage.PercpuUsageInKernelmode
	s.CPU.Usage.PercpuUser = cg.CpuStats.CpuUsage.PercpuUsageInUsermode
	s.CPU.Throttling.Periods = cg.CpuStats.ThrottlingData.Periods
	s.CPU.Throttling.ThrottledPeriods = cg.CpuStats.ThrottlingData.ThrottledPeriods
	s.CPU.Throttling.ThrottledTime = cg.CpuStats.ThrottlingData.ThrottledTime

	s.Memory.Cache = cg.MemoryStats.Cache
	s.Memory.Kernel = convertMemoryEntry(cg.MemoryStats.KernelUsage)
	s.Memory.KernelTCP = convertMemoryEntry(cg.MemoryStats.KernelTCPUsage)
	s.Memory.Swap = convertMemoryEntry(cg.MemoryStats.SwapUsage)
	s.Memory.Usage = convertMemoryEntry(cg.MemoryStats.Usage)
	s.Memory.Raw = cg.MemoryStats.Stats

	s.Blkio.IoServiceBytesRecursive = convertBlkioEntry(cg.BlkioStats.IoServiceBytesRecursive)
	s.Blkio.IoServicedRecursive = convertBlkioEntry(cg.BlkioStats.IoServicedRecursive)
	s.Blkio.IoQueuedRecursive = convertBlkioEntry(cg.BlkioStats.IoQueuedRecursive)
	s.Blkio.IoServiceTimeRecursive = convertBlkioEntry(cg.BlkioStats.IoServiceTimeRecursive)
	s.Blkio.IoWaitTimeRecursive = convertBlkioEntry(cg.BlkioStats.IoWaitTimeRecursive)
	s.Blkio.IoMergedRecursive = convertBlkioEntry(cg.BlkioStats.IoMergedRecursive)
	s.Blkio.IoTimeRecursive = convertBlkioEntry(cg.BlkioStats.IoTimeRecursive)
	s.Blkio.SectorsRecursive = convertBlkioEntry(cg.BlkioStats.SectorsRecursive)

	s.Hugetlb = make(map[string]types.Hugetlb)
	for k, v := range cg.HugetlbStats {
		s.Hugetlb[k] = convertHugtlb(v)
	}

	if is := ls.IntelRdtStats; is != nil {
		if intelrdt.IsCatEnabled() {
			s.IntelRdt.L3CacheInfo = convertL3CacheInfo(is.L3CacheInfo)
			s.IntelRdt.L3CacheSchemaRoot = is.L3CacheSchemaRoot
			s.IntelRdt.L3CacheSchema = is.L3CacheSchema
		}
		if intelrdt.IsMbaEnabled() {
			s.IntelRdt.MemBwInfo = convertMemBwInfo(is.MemBwInfo)
			s.IntelRdt.MemBwSchemaRoot = is.MemBwSchemaRoot
			s.IntelRdt.MemBwSchema = is.MemBwSchema
		}
		if intelrdt.IsMBMEnabled() {
			s.IntelRdt.MBMStats = is.MBMStats
		}
		if intelrdt.IsCMTEnabled() {
			s.IntelRdt.CMTStats = is.CMTStats
		}
	}

	s.NetworkInterfaces = ls.Interfaces
	return &s
}

func convertHugtlb(c cgroups.HugetlbStats) types.Hugetlb {
	return types.Hugetlb{
		Usage:   c.Usage,
		Max:     c.MaxUsage,
		Failcnt: c.Failcnt,
	}
}

func convertMemoryEntry(c cgroups.MemoryData) types.MemoryEntry {
	return types.MemoryEntry{
		Limit:   c.Limit,
		Usage:   c.Usage,
		Max:     c.MaxUsage,
		Failcnt: c.Failcnt,
	}
}

func convertBlkioEntry(c []cgroups.BlkioStatEntry) []types.BlkioEntry {
	var out []types.BlkioEntry
	for _, e := range c {
		out = append(out, types.BlkioEntry{
			Major: e.Major,
			Minor: e.Minor,
			Op:    e.Op,
			Value: e.Value,
		})
	}
	return out
}

func convertL3CacheInfo(i *intelrdt.L3CacheInfo) *types.L3CacheInfo {
	return &types.L3CacheInfo{
		CbmMask:    i.CbmMask,
		MinCbmBits: i.MinCbmBits,
		NumClosids: i.NumClosids,
	}
}

func convertMemBwInfo(i *intelrdt.MemBwInfo) *types.MemBwInfo {
	return &types.MemBwInfo{
		BandwidthGran: i.BandwidthGran,
		DelayLinear:   i.DelayLinear,
		MinBandwidth:  i.MinBandwidth,
		NumClosids:    i.NumClosids,
	}
}
