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

func abbrevPubKey(pubkey string) string {
	// ll := len(pubkey)
	// return pubkey[0:4] + ".." + pubkey[ll-4:ll]
	return pubkey
}

func listChannels(client lnrpc.LightningClient, ctx context.Context) {
	info, err := client.GetInfo(ctx, &lnrpc.GetInfoRequest{})
    if err != nil {
		panic(fmt.Sprint("GetInfo failed:", err))
    }
	
	rsp, err := client.ListChannels(ctx, &lnrpc.ListChannelsRequest{
		ActiveOnly: false,
		InactiveOnly: false,
		PublicOnly: false,
		PrivateOnly: false,
	})
    if err != nil {
		panic(fmt.Sprint("ListChannels failed:", err))
    }

	color.Bold.Println("ChanId             Flg  Capacity     Local    Remote  Imbalance PubKey                                                             Log Alias")
		
	sumCapacity := int64(0)
	sumLocal := int64(0)
	sumRemote := int64(0)
	sort.SliceStable(rsp.Channels, func(ii, jj int) bool {
		return rsp.Channels[ii].ChanId < rsp.Channels[jj].ChanId
	})
	for _, chn := range rsp.Channels {
		nodeInfo, err := client.GetNodeInfo(ctx, &lnrpc.NodeInfoRequest{
			PubKey: chn.RemotePubkey,
		})
		if err != nil {
			panic(fmt.Sprint("GetNodeInfo failed:", err))
		}
		rmtCap := nodeInfo.TotalCapacity
		alias := nodeInfo.Node.Alias
		
		chanInfo, err := client.GetChanInfo(ctx, &lnrpc.ChanInfoRequest{
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
		
		str := fmt.Sprintf("%d %s%s%s %9d %9d %9d %10d %s %3.1f %s",
			chn.ChanId,
			initiator,
			active,
			disabled,
			chn.Capacity,
			chn.LocalBalance,
			chn.RemoteBalance,
			imbalance,
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

	pendingChannels, err := client.PendingChannels(ctx, &lnrpc.PendingChannelsRequest{})
    if err != nil {
		panic(fmt.Sprint("PendingChannels failed:", err))
    }
	for _, chn2 := range pendingChannels.PendingOpenChannels {
		disabled := "o"
		initiator := "o"
		active := "o"

		nodeInfo, err := client.GetNodeInfo(ctx, &lnrpc.NodeInfoRequest{
			PubKey: chn2.Channel.RemoteNodePub,
		})
		if err != nil {
			panic(fmt.Sprint("GetNodeInfo failed:", err))
		}
		rmtCap := nodeInfo.TotalCapacity
		alias := nodeInfo.Node.Alias
		
		imbalance := chn2.Channel.LocalBalance -
			((chn2.Channel.LocalBalance + chn2.Channel.RemoteBalance) / 2)
		
		fmt.Printf("                   %s%s%s %9d %9d %9d %10d %s %3.1f %s\n",
			initiator,
			active,
			disabled,
			chn2.Channel.Capacity,
			chn2.Channel.LocalBalance,
			chn2.Channel.RemoteBalance,
			imbalance,
			abbrevPubKey(chn2.Channel.RemoteNodePub),
			math.Log10(float64(rmtCap)),
			alias,
		)
		sumCapacity += chn2.Channel.Capacity
		sumLocal += chn2.Channel.LocalBalance
		sumRemote += chn2.Channel.RemoteBalance
	}
	
	imbalance := sumLocal - ((sumLocal + sumRemote) / 2)
	
	color.Bold.Printf("%2d                     %9d %9d %9d %10d %s %3.1f %s\n",
		len(rsp.Channels) + len(pendingChannels.PendingOpenChannels),
		sumCapacity,
		sumLocal,
		sumRemote,
		imbalance,
		abbrevPubKey(info.IdentityPubkey),
		math.Log10(float64(sumCapacity)),
		info.Alias,
	)
}
