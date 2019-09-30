// Copyright 2019 Bonsai Software, Inc.  All Rights Reserved.

package main

import (
	"encoding/csv"
	"fmt"
	"math"
	"os"
	"strconv"

	"github.com/lightningnetwork/lnd/lnrpc"
)

type nodeStats struct {
	PubKey        string
	NumChannels   int
	TotalCapacity int64
	WeightedBase  float64
	WeightedRate  float64
}

func graphStats() {

	// Setup X parameters
	minLogX := math.Log10(float64(1e3))
	logStepX := float64(0.2)
	logOffX := logStepX / 2
	nBuckX := 31

	// Allocate X threshold array
	threshX := make([]float64, nBuckX)

	// Fill the X threshold array
	xx := minLogX
	for ndx := 0; ndx < nBuckX; ndx += 1 {
		threshX[ndx] = math.Pow(10, xx-logOffX)
		xx += logStepX
	}

	// for ndx, xx := range threshX {
	//     fmt.Println(ndx, xx)
	// }

	// Setup Y parameters
	minLogY := math.Log10(float64(0.01))
	logStepY := float64(0.2)
	logOffY := logStepY / 2
	nBuckY := 26

	// Allocate Y threshold array
	threshY := make([]float64, nBuckY)

	// Fill the Y threshold array
	yy := minLogY
	for ndx := 0; ndx < nBuckY; ndx += 1 {
		threshY[ndx] = math.Pow(10, yy-logOffY)
		yy += logStepY
	}

	// for ndx, yy := range threshY {
	//     fmt.Println(ndx, yy)
	// }

	// Allocate counts array.
	chanRateCap := make([][]int, nBuckX)
	for ndx := range chanRateCap {
		chanRateCap[ndx] = make([]int, nBuckY)
	}

	nsfile, err := os.Create("nodestats.csv")
	if err != nil {
		fmt.Println("os.Create", "nodestats.csv", "failed:", err)
		return
	}
	defer nsfile.Close()

	nswriter := csv.NewWriter(nsfile)
	defer nswriter.Flush()

	// Write the CSV header
	row := []string{"pubkey", "nchannels", "capacity", "base", "rate", "alias"}
	err = nswriter.Write(row)

	csfile, err := os.Create("chanstats.csv")
	if err != nil {
		fmt.Println("os.Create", "chanstats.csv", "failed:", err)
		return
	}
	defer csfile.Close()

	cswriter := csv.NewWriter(csfile)
	defer cswriter.Flush()

	// Write the CSV header
	row = []string{"chanid", "capacity", "base", "rate", "alias"}
	err = cswriter.Write(row)

	rcfile, err := os.Create("ratevcap.csv")
	if err != nil {
		fmt.Println("os.Create", "ratevcap.csv", "failed:", err)
		return
	}
	defer rcfile.Close()

	rcwriter := csv.NewWriter(rcfile)
	defer rcwriter.Flush()

	// Write the CSV header
	row = []string{"capacity", "rate", "count"}
	err = rcwriter.Write(row)

	channelGraph, err := gClient.DescribeGraph(gCtx,
		&lnrpc.ChannelGraphRequest{},
	)
	if err != nil {
		fmt.Println("DescribeGraph failed:", err)
		return
	}

	numNodes := int(0)
	numChannels := int(0)
	missingPolicy := int(0)

	for _, node := range channelGraph.Nodes {
		numNodes += 1

		if gCfg.Verbose {
			fmt.Println(node.PubKey)
		}
		nodeInfo, err := gClient.GetNodeInfo(gCtx,
			&lnrpc.NodeInfoRequest{
				PubKey:          node.PubKey,
				IncludeChannels: true,
			},
		)
		if err != nil {
			fmt.Println("GetNodeInfo", node.PubKey, "failed:", err)
			return
		}

		if gCfg.Verbose {
			fmt.Println(nodeInfo.Node.Alias)
		}

		nodeChannels := int(0)
		nodeCapacity := float64(0)
		prodBase := float64(0)
		prodRate := float64(0)
		for _, channelEdge := range nodeInfo.Channels {
			if true {
				// WORKAROUND - Seems the Node1Policy/Node2Policy
				// are twisted when IncludeChannels is used?
				// Instead manually fetch the channel info.
				// SEE: https://github.com/lightningnetwork/lnd/issues/3426
				//
				channelEdge, err = gClient.GetChanInfo(gCtx,
					&lnrpc.ChanInfoRequest{
						ChanId: channelEdge.ChannelId,
					},
				)
				if err != nil {
					fmt.Println("GetChanInfo",
						channelEdge.ChannelId, "failed:", err)
					return
				}
			}

			// Ensure we are match one direction exactly
			isNode1 := node.PubKey == channelEdge.Node1Pub
			isNode2 := node.PubKey == channelEdge.Node2Pub
			if isNode1 != !isNode2 {
				panic("isNode1 != !isNode2")
			}

			// Which policy is in effect here?
			var policy *lnrpc.RoutingPolicy
			if isNode1 {
				policy = channelEdge.Node1Policy
			} else {
				policy = channelEdge.Node2Policy
			}
			// Sometimes there is no policy?
			if policy == nil {
				missingPolicy += 1
				continue
			}

			numChannels += 1
			nodeChannels += 1

			chanBase := float64((*policy).FeeBaseMsat) / 1000     // sat
			chanRate := float64((*policy).FeeRateMilliMsat) / 100 // bps
			row := []string{
				strconv.FormatUint(channelEdge.ChannelId, 10), // channel id
				strconv.FormatInt(channelEdge.Capacity, 10),   // capacity
				strconv.FormatFloat(chanBase, 'f', -1, 64),    // base
				strconv.FormatFloat(chanRate, 'f', -1, 64),    // rate
				nodeInfo.Node.Alias,                           // alias
			}
			_ = cswriter.Write(row)

			chanCap := float64(channelEdge.Capacity)
			histXX := -1
			if chanCap >= threshX[0] {
				for ndx := 1; ndx < nBuckX; ndx++ {
					if chanCap < threshX[ndx] {
						histXX = ndx - 1
						break
					}
				}
			}
			histYY := -1
			if chanRate >= threshY[0] {
				for ndx := 1; ndx < nBuckY; ndx++ {
					if chanRate < threshY[ndx] {
						histYY = ndx - 1
						break
					}
				}
			}

			if histXX != -1 && histYY != -1 {
				// fmt.Println(histXX, histYY)
				chanRateCap[histXX][histYY] += 1
			}

			// Aggregate the capacity and weighted base and rate.
			fmt.Println(
				isNode1,
				isNode2,
				channelEdge.ChannelId,
				channelEdge.Capacity,
				(*policy).FeeBaseMsat,
				(*policy).FeeRateMilliMsat,
			)
			nodeCapacity += float64(channelEdge.Capacity)
			prodBase += float64(channelEdge.Capacity) *
				float64((*policy).FeeBaseMsat)
			prodRate += float64(channelEdge.Capacity) *
				float64((*policy).FeeRateMilliMsat)
		}
		if nodeCapacity > 0 {
			weightedBase := (prodBase / nodeCapacity) / 1000 // sat
			weightedRate := (prodRate / nodeCapacity) / 100  // bps
			if gCfg.Verbose {
				fmt.Println("NodeChannels", nodeChannels)
				fmt.Println("NodeCapacity", nodeCapacity)
				fmt.Println("WeightedBase", weightedBase, "sat")
				fmt.Println("WeightedRate", weightedRate, "bps")
			}

			row := []string{
				node.PubKey,                // pubkey
				strconv.Itoa(nodeChannels), // nchannels
				strconv.FormatFloat(nodeCapacity, 'f', -1, 64), // capacity
				strconv.FormatFloat(weightedBase, 'f', -1, 64), // base
				strconv.FormatFloat(weightedRate, 'f', -1, 64), // rate
				nodeInfo.Node.Alias,                            // alias
			}
			_ = nswriter.Write(row)
		}
		fmt.Println()
	}
	if gCfg.Verbose {
		fmt.Println("NumNodes", numNodes)
		fmt.Println("NumChannels", numChannels)
		fmt.Println("MissingPolicy", missingPolicy)
	}

	xLogVal := minLogX
	for xx := 0; xx < nBuckX; xx++ {
		yLogVal := minLogY
		for yy := 0; yy < nBuckY; yy++ {
			fmt.Println(xx, yy, chanRateCap[xx][yy],
				math.Pow(10, xLogVal), math.Pow(10, yLogVal))
			row := []string{
				strconv.FormatFloat(math.Pow(10, xLogVal), 'f', -1, 64),
				strconv.FormatFloat(math.Pow(10, yLogVal), 'f', -1, 64),
				strconv.Itoa(chanRateCap[xx][yy]),
			}
			_ = rcwriter.Write(row)
			yLogVal += logStepY
		}
		xLogVal += logStepX
	}
}
