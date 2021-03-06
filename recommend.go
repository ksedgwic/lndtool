// Copyright 2019 Bonsai Software, Inc.  All Rights Reserved.

package main

import (
	"fmt"
	"sort"
	"time"

	"github.com/lightningnetwork/lnd/lnrpc"
)

type NodeBalance struct {
	LocalBalance  int64
	RemoteBalance int64
}

type PotentialLoop struct {
	SrcChan uint64
	SrcNode string
	DstChan uint64
	DstNode string
	Amount  int64
}

func NewPotentialLoop(
	srcChan uint64,
	srcNode string,
	dstChan uint64,
	dstNode string,
	amount int64,
) *PotentialLoop {
	return &PotentialLoop{
		SrcChan: srcChan,
		SrcNode: srcNode,
		DstChan: dstChan,
		DstNode: dstNode,
		Amount:  amount,
	}
}

func recommend(doit bool) bool {

	var blacklist = map[string]bool{}
	for _, node := range gCfg.Recommend.PeerNodeBlacklist {
		blacklist[node] = true
	}
	var srclist = map[uint64]bool{}
	for _, node := range gCfg.Recommend.SrcChanTarget {
		srclist[node] = true
	}
	var dstlist = map[uint64]bool{}
	for _, node := range gCfg.Recommend.DstChanTarget {
		dstlist[node] = true
	}

	rsp, err := gClient.ListChannels(gCtx, &lnrpc.ListChannelsRequest{
		ActiveOnly:   true,
		InactiveOnly: false,
		PublicOnly:   true,
		PrivateOnly:  false,
	})
	if err != nil {
		panic(fmt.Sprint("ListChannels failed:", err))
	}

	// Aggregate local and remote balances per node (matters when
	// there are multiple channels to the same node.
	//
	nodeBalances := map[string]*NodeBalance{}
	for _, nodeChan := range rsp.Channels {
		nb := nodeBalances[nodeChan.RemotePubkey]
		if nb == nil {
			nb = &NodeBalance{0, 0}
			nodeBalances[nodeChan.RemotePubkey] = nb
		}
		nb.LocalBalance += nodeChan.LocalBalance
		nb.RemoteBalance += nodeChan.RemoteBalance
	}

	// Consider all combinations of channels
	loops := []*PotentialLoop{}
	for srcNdx, srcChan := range rsp.Channels {

		// Is this node blacklisted?
		if blacklist[srcChan.RemotePubkey] {
			continue
		}

		// Is there a source list?  Is this channel in it?
		if len(srclist) > 0 && !srclist[srcChan.ChanId] {
			continue
		}

		for dstNdx, dstChan := range rsp.Channels {

			// Is this node blacklisted?
			if blacklist[dstChan.RemotePubkey] {
				continue
			}

			// Is there a dest list?  Is this channel in it?
			if len(dstlist) > 0 && !dstlist[dstChan.ChanId] {
				continue
			}

			// Same channel can't be both src and dst:
			if srcNdx == dstNdx {
				continue
			}

			// Make sure the connected nodes are unique:
			if srcChan.RemotePubkey == dstChan.RemotePubkey {
				continue
			}

			// Make sure the aggregate source node imbalance is positive:
			aggSrcImbalance :=
				nodeBalances[srcChan.RemotePubkey].LocalBalance -
					((nodeBalances[srcChan.RemotePubkey].LocalBalance +
						nodeBalances[srcChan.RemotePubkey].RemoteBalance) / 2)
			if aggSrcImbalance < gCfg.Recommend.MinImbalance {
				continue
			}

			// Make sure the aggregate dest node imbalance is negative:
			aggDstImbalance :=
				nodeBalances[dstChan.RemotePubkey].LocalBalance -
					((nodeBalances[dstChan.RemotePubkey].LocalBalance +
						nodeBalances[dstChan.RemotePubkey].RemoteBalance) / 2)
			if aggDstImbalance > -gCfg.Recommend.MinImbalance {
				continue
			}

			// Make sure the specific source imbalance is positive:
			srcImbalance :=
				srcChan.LocalBalance -
					((srcChan.LocalBalance + srcChan.RemoteBalance) / 2)
			if srcImbalance < gCfg.Recommend.MinImbalance {
				continue
			}

			// Make sure the specific destination imbalance is negative:
			dstImbalance :=
				dstChan.LocalBalance -
					((dstChan.LocalBalance + dstChan.RemoteBalance) / 2)
			if dstImbalance > -gCfg.Recommend.MinImbalance {
				continue
			}

			// What is the target amount to be moved?
			amount := int64(0)
			if aggSrcImbalance < -aggDstImbalance {
				amount = aggSrcImbalance
			} else {
				amount = -aggDstImbalance
			}

			loops = append(loops, NewPotentialLoop(
				srcChan.ChanId, srcChan.RemotePubkey,
				dstChan.ChanId, dstChan.RemotePubkey,
				amount,
			))
		}
	}

	sort.SliceStable(loops, func(ii, jj int) bool {
		// Amount descending
		return loops[ii].Amount > loops[jj].Amount
	})

	for _, loop := range loops {
		// Limit the rebalance amount
		amount := loop.Amount
		if amount > gCfg.Recommend.TransferAmount {
			amount = gCfg.Recommend.TransferAmount
		}

		// Consider recent history
		tstamp := time.Now().Unix() - int64(gCfg.Recommend.RetryInhibit.Seconds())
		if !recentlyFailed(loop.SrcChan, loop.DstChan, tstamp, amount, gCfg.Rebalance.FeeLimitRate) {
			if doit {
				doRebalance(amount, loop.SrcChan, loop.DstChan)
				return true
			} else {
				fmt.Printf("lndtool rebalance -a %d -s %d -d %d\n",
					amount, loop.SrcChan, loop.DstChan)
				return true
			}
		}
	}

	fmt.Println("no loops recommended")
	return false
}
