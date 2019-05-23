// Copyright 2019 Bonsai Software, Inc.  All Rights Reserved.

package main

import (
	"context"
	"fmt"
	"sort"
	
    "github.com/lightningnetwork/lnd/lnrpc"
)

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

		rsp2,err := client.GetChanInfo(ctx, &lnrpc.ChanInfoRequest{
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
			
		fmt.Printf("%d %s %10d %8d %8d %4.1f%% %s %s %s\n",
			chn.ChanId,
			chn.RemotePubkey,
			chn.Capacity,
			chn.LocalBalance,
			chn.RemoteBalance,
			lclpct,
			initiator,
			active,
			disabled,
		)
		sumCapacity += chn.Capacity
		sumLocal += chn.LocalBalance
		sumRemote += chn.RemoteBalance
	}
	fmt.Printf("                                                                                      %10d %8d %8d\n",
		sumCapacity,
		sumLocal,
		sumRemote,
	)
}
