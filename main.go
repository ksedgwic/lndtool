// Copyright 2019 Bonsai Software, Inc.  All Rights Reserved.

package main

import (
    "context"
    "fmt"
    "io/ioutil"
    "os/user"
    "path"
	
    "github.com/lightningnetwork/lnd/lnrpc"
    "github.com/lightningnetwork/lnd/macaroons"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials"
    "gopkg.in/macaroon.v2"
)

func main() {
    usr, err := user.Current()
    if err != nil {
        fmt.Println("Cannot get current user:", err)
        return
    }
    tlsCertPath := path.Join(usr.HomeDir, ".lnd/tls.cert")
    macaroonPath := path.Join(usr.HomeDir,
		".lnd/data/chain/bitcoin/mainnet/admin.macaroon")

    tlsCreds, err := credentials.NewClientTLSFromFile(tlsCertPath, "")
    if err != nil {
        fmt.Println("Cannot get node tls credentials", err)
        return
    }

    macaroonBytes, err := ioutil.ReadFile(macaroonPath)
    if err != nil {
        fmt.Println("Cannot read macaroon file", err)
        return
    }

    mac := &macaroon.Macaroon{}
    if err = mac.UnmarshalBinary(macaroonBytes); err != nil {
        fmt.Println("Cannot unmarshal macaroon", err)
        return
    }

    opts := []grpc.DialOption{
        grpc.WithTransportCredentials(tlsCreds),
        grpc.WithBlock(),
        grpc.WithPerRPCCredentials(macaroons.NewMacaroonCredential(mac)),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(1 * 1024 * 1024 * 50)),
    }

    conn, err := grpc.Dial("localhost:10009", opts...)
    if err != nil {
        fmt.Println("cannot dial to lnd", err)
        return
    }
    client := lnrpc.NewLightningClient(conn)
	ctx := context.Background()

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
