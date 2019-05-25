// Copyright 2019 Bonsai Software, Inc.  All Rights Reserved.

package main

import (
	"context"
	"fmt"
	"math"
	"sort"
	
	"github.com/gookit/color"
    "github.com/lightningnetwork/lnd/lnrpc"
)

func nodeAlias(client lnrpc.LightningClient, ctx context.Context, pubkey string) string {
	rsp, err := client.GetNodeInfo(ctx, &lnrpc.NodeInfoRequest{
		PubKey: pubkey,
	})
	if err != nil {
		panic(fmt.Sprint("GetChanInfo failed:", err))
	}
	return rsp.Node.Alias
}

func listChannels(client lnrpc.LightningClient, ctx context.Context) {
	rsp, err := client.ListChannels(ctx, &lnrpc.ListChannelsRequest{
		ActiveOnly: false,
		InactiveOnly: false,
		PublicOnly: false,
		PrivateOnly: false,
	})
    if err != nil {
		panic(fmt.Sprint("ListChannels failed:", err))
    }

	sumCapacity := int64(0)
	sumLocal := int64(0)
	sumRemote := int64(0)
	sort.SliceStable(rsp.Channels, func(ii, jj int) bool {
		return rsp.Channels[ii].ChanId < rsp.Channels[jj].ChanId
	})
	for _, chn := range rsp.Channels {
		rsp1, err := client.GetNodeInfo(ctx, &lnrpc.NodeInfoRequest{
			PubKey: chn.RemotePubkey,
		})
		if err != nil {
			panic(fmt.Sprint("GetNodeInfo failed:", err))
		}
		rmtCap := rsp1.TotalCapacity
		
		rsp2, err := client.GetChanInfo(ctx, &lnrpc.ChanInfoRequest{
			ChanId: chn.ChanId,
		})
		if err != nil {
			panic(fmt.Sprint("GetChanInfo failed:", err))
		}
		var policy *lnrpc.RoutingPolicy
		if rsp2.Node1Pub == chn.RemotePubkey {
			policy = rsp2.Node2Policy
		} else {
			policy = rsp2.Node1Policy
		}
		var disabled string
		if policy.Disabled {
			disabled = "D"
		} else {
			disabled = "E"
		}
		
		lclpct := 100.0 * float64(chn.LocalBalance) / float64(chn.Capacity)
		
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

		alias := nodeAlias(client, ctx, chn.RemotePubkey)
		logRmtCap := math.Log10(float64(rmtCap))
			
		str := fmt.Sprintf("%d %s %9d %9d %9d %4.1f%% %s %s %s %3.1f %s",
			chn.ChanId,
			chn.RemotePubkey,
			chn.Capacity,
			chn.LocalBalance,
			chn.RemoteBalance,
			lclpct,
			initiator,
			active,
			disabled,
			logRmtCap,
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

	rsp2, err := client.PendingChannels(ctx, &lnrpc.PendingChannelsRequest{})
    if err != nil {
		panic(fmt.Sprint("PendingChannels failed:", err))
    }
	for _, chn2 := range rsp2.PendingOpenChannels {
		fmt.Printf("                   %s %9d %9d %9d             %3.1f\n",
			chn2.Channel.RemoteNodePub,
			chn2.Channel.Capacity,
			chn2.Channel.LocalBalance,
			chn2.Channel.RemoteBalance,
			math.Log10(float64(chn2.Channel.Capacity)),
		)
		sumCapacity += chn2.Channel.Capacity
		sumLocal += chn2.Channel.LocalBalance
		sumRemote += chn2.Channel.RemoteBalance
	}
	
	fmt.Printf("%2d                                                                                    %9d %9d %9d             %3.1f\n",
		len(rsp.Channels) + len(rsp2.PendingOpenChannels),
		sumCapacity,
		sumLocal,
		sumRemote,
		math.Log10(float64(sumCapacity)),
	)
}
