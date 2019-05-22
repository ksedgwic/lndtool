// Copyright 2019 Bonsai Software, Inc.  All Rights Reserved.

package main

import (
	"context"
	"fmt"
	
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
        fmt.Println("ListChannels failed:", err)
        return
    }

	sumCapacity := int64(0)
	sumLocal := int64(0)
	sumRemote := int64(0)
	for _, chn := range rsp.Channels {
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
			
		fmt.Printf("%d %s %10d %8d %8d %4.1f%% %s %s\n",
			chn.ChanId,
			chn.RemotePubkey,
			chn.Capacity,
			chn.LocalBalance,
			chn.RemoteBalance,
			lclpct,
			initiator,
			active,
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
