// Copyright 2019 Bonsai Software, Inc.  All Rights Reserved.

package main

import (
	"fmt"
	"math"
	"sort"
	"time"

	// "github.com/davecgh/go-spew/spew"
	"github.com/gookit/color"
	"github.com/lightningnetwork/lnd/lnrpc"
)

type FwdStatsElem struct {
	CountRcv   uint64
	AmountRcv  uint64
	FeeMsatRcv uint64

	CountSnd   uint64
	AmountSnd  uint64
	FeeMsatSnd uint64
}

type FwdStats map[uint64]*FwdStatsElem

func getFwdStats() *FwdStats {
	retval := FwdStats{}

	hist, err := gClient.ForwardingHistory(gCtx, &lnrpc.ForwardingHistoryRequest{
		StartTime: uint64((time.Now().Add(-gCfg.Channels.StatsWindow)).Unix()),
		EndTime:   uint64(time.Now().Unix()),
	})
	if err != nil {
		panic(fmt.Sprint("ForwardingHistory failed: %v\n", err))
	}

	for _, evt := range hist.ForwardingEvents {
		rcvElem, ok := retval[evt.ChanIdIn]
		if !ok {
			rcvElem = &FwdStatsElem{}
			retval[evt.ChanIdIn] = rcvElem
		}
		sndElem, ok := retval[evt.ChanIdOut]
		if !ok {
			sndElem = &FwdStatsElem{}
			retval[evt.ChanIdOut] = sndElem
		}

		rcvElem.CountRcv += 1
		rcvElem.AmountRcv += evt.AmtIn
		rcvElem.FeeMsatRcv += evt.FeeMsat

		sndElem.CountSnd += 1
		sndElem.AmountSnd += evt.AmtOut
		sndElem.FeeMsatSnd += evt.FeeMsat
	}

	return &retval
}

func abbrevPubKey(pubkey string) string {
	// ll := len(pubkey)
	// return pubkey[0:4] + ".." + pubkey[ll-4:ll]
	return pubkey
}

func fmtAmountSci(amt float64) string {
	if amt > 0 {
		buf := fmt.Sprintf("%7.1e", amt)
		if buf[len(buf)-3:len(buf)-1] == "+0" {
			buf = buf[:len(buf)-3] + buf[len(buf)-1:]
		}
		return buf
	} else {
		return "0    "
	}
}

func listChannels() {

	fwdStats := getFwdStats()

	info, err := gClient.GetInfo(gCtx, &lnrpc.GetInfoRequest{})
	if err != nil {
		panic(fmt.Sprint("GetInfo failed: %v\n", err))
	}

	rsp, err := gClient.ListChannels(gCtx, &lnrpc.ListChannelsRequest{
		ActiveOnly:   false,
		InactiveOnly: false,
		PublicOnly:   false,
		PrivateOnly:  false,
	})
	if err != nil {
		panic(fmt.Sprint("ListChannels failed:", err))
	}

	color.Bold.Println("ChanId             Flg  Capacity     Local    Remote  Imbalance FwdR  FwdS  PubKey                                                             Log Alias")

	sumCapacity := int64(0)
	sumLocal := int64(0)
	sumRemote := int64(0)
	sumFwdRcv := uint64(0)
	sumFwdSnd := uint64(0)
	sort.SliceStable(rsp.Channels, func(ii, jj int) bool {
		return rsp.Channels[ii].ChanId < rsp.Channels[jj].ChanId
	})
	for _, chn := range rsp.Channels {
		nodeInfo, err := gClient.GetNodeInfo(gCtx, &lnrpc.NodeInfoRequest{
			PubKey: chn.RemotePubkey,
		})
		if err != nil {
			panic(fmt.Sprint("GetNodeInfo failed:", err))
		}
		rmtCap := nodeInfo.TotalCapacity
		alias := nodeInfo.Node.Alias

		chanInfo, err := gClient.GetChanInfo(gCtx, &lnrpc.ChanInfoRequest{
			ChanId: chn.ChanId,
		})
		if err != nil {
			panic(fmt.Sprint("GetChanInfo failed:", err))
		}
		var policy *lnrpc.RoutingPolicy
		if chanInfo.Node1Pub == chn.RemotePubkey {
			policy = chanInfo.Node2Policy
		} else {
			policy = chanInfo.Node1Policy
		}
		var disabled string
		if policy.Disabled {
			disabled = "D"
		} else {
			disabled = "E"
		}

		var initiator string
		if chn.Initiator {
			initiator = "L"
		} else {
			initiator = "R"
		}

		var active string
		if chn.Active {
			active = "A"
		} else {
			active = "I"
		}

		imbalance := chn.LocalBalance -
			((chn.LocalBalance + chn.RemoteBalance) / 2)

		chanStats := chanStats(chn.ChanId)
		chnStatsStr := fmt.Sprintf("%1.0f %1.0f %1.0f %1.0f %1.0f %1.0f",
			math.Log10(float64(chanStats.RcvCnt+1)),
			math.Log10(float64(chanStats.RcvErr+1)),
			math.Log10(float64(chanStats.RcvSat+1)),
			math.Log10(float64(chanStats.SndCnt+1)),
			math.Log10(float64(chanStats.SndErr+1)),
			math.Log10(float64(chanStats.SndSat+1)),
		)
		_ = chnStatsStr

		chnFwdStats := (*fwdStats)[chn.ChanId]
		if chnFwdStats == nil {
			chnFwdStats = &FwdStatsElem{}
		}
		chnFwdStatsStr := fmt.Sprintf("%s %s",
			fmtAmountSci(float64(chnFwdStats.AmountRcv)),
			fmtAmountSci(float64(chnFwdStats.AmountSnd)),
		)
		sumFwdRcv += chnFwdStats.AmountRcv
		sumFwdSnd += chnFwdStats.AmountSnd

		str := fmt.Sprintf("%d %s%s%s %9d %9d %9d %10d %s %s %3.1f %s",
			chn.ChanId,
			initiator,
			active,
			disabled,
			chn.Capacity,
			chn.LocalBalance,
			chn.RemoteBalance,
			imbalance,
			// chnStatsStr,
			chnFwdStatsStr,
			abbrevPubKey(chn.RemotePubkey),
			math.Log10(float64(rmtCap)),
			alias,
		)

		if policy.Disabled {
			color.Red.Println(str)
		} else if !chn.Active {
			color.Yellow.Println(str)
		} else {
			color.Black.Println(str)
		}

		sumCapacity += chn.Capacity
		sumLocal += chn.LocalBalance
		sumRemote += chn.RemoteBalance
	}

	pendingChannels, err := gClient.PendingChannels(gCtx, &lnrpc.PendingChannelsRequest{})
	if err != nil {
		panic(fmt.Sprint("PendingChannels failed:", err))
	}
	for _, chn2 := range pendingChannels.PendingOpenChannels {
		disabled := "o"
		initiator := "o"
		active := "o"

		nodeInfo, err := gClient.GetNodeInfo(gCtx, &lnrpc.NodeInfoRequest{
			PubKey: chn2.Channel.RemoteNodePub,
		})
		if err != nil {
			panic(fmt.Sprint("GetNodeInfo failed:", err))
		}
		rmtCap := nodeInfo.TotalCapacity
		alias := nodeInfo.Node.Alias

		chnFwdStatsStr := fmt.Sprintf("%s %s",
			fmtAmountSci(float64(0)),
			fmtAmountSci(float64(0)),
		)

		imbalance := chn2.Channel.LocalBalance -
			((chn2.Channel.LocalBalance + chn2.Channel.RemoteBalance) / 2)

		fmt.Printf("                   %s%s%s %9d %9d %9d %10d %s %s %3.1f %s\n",
			initiator,
			active,
			disabled,
			chn2.Channel.Capacity,
			chn2.Channel.LocalBalance,
			chn2.Channel.RemoteBalance,
			imbalance,
			chnFwdStatsStr,
			abbrevPubKey(chn2.Channel.RemoteNodePub),
			math.Log10(float64(rmtCap)),
			alias,
		)
		sumCapacity += chn2.Channel.Capacity
		sumLocal += chn2.Channel.LocalBalance
		sumRemote += chn2.Channel.RemoteBalance
	}

	chnFwdStatsStr := fmt.Sprintf("%s %s",
		fmtAmountSci(float64(sumFwdRcv)),
		fmtAmountSci(float64(sumFwdSnd)),
	)

	imbalance := sumLocal - ((sumLocal + sumRemote) / 2)

	color.Bold.Printf("%2d                     %9d %9d %9d %10d %s %s %3.1f %s\n",
		len(rsp.Channels)+len(pendingChannels.PendingOpenChannels),
		sumCapacity,
		sumLocal,
		sumRemote,
		imbalance,
		chnFwdStatsStr,
		abbrevPubKey(info.IdentityPubkey),
		math.Log10(float64(sumCapacity)),
		info.Alias,
	)
}
