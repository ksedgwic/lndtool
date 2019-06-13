// Copyright 2019 Bonsai Software, Inc.  All Rights Reserved.

package main

import (
	"context"
	"fmt"
	"math"
	"sort"

	"github.com/lightningnetwork/lnd/lnrpc"
)

type Edge struct {
	lnrpc.ChannelEdge
}

func NewEdge(ee *lnrpc.ChannelEdge) *Edge {
	return &Edge{ChannelEdge: *ee}
}

type Node struct {
	lnrpc.LightningNode
	Edges         []*Edge
	Policy        []*lnrpc.RoutingPolicy
	Peers         []*Node
	NumHops       int
	CumulativeFee float64
}

func NewNode(nn *lnrpc.LightningNode) *Node {
	return &Node{
		LightningNode: *nn,
		Edges:         []*Edge{},
		NumHops:       -1,
		CumulativeFee: 0,
	}
}

func (node *Node) AddEdge(edge *Edge, policy *lnrpc.RoutingPolicy, peer *Node) {
	if policy != nil && !policy.Disabled {
		node.Edges = append(node.Edges, edge)
		node.Policy = append(node.Policy, policy)
		node.Peers = append(node.Peers, peer)
	}
}

func (node *Node) NumChan() int {
	return len(node.Peers)
}

func (node *Node) Capacity() int64 {
	sum := int64(0)
	for _, ee := range node.Edges {
		sum += ee.Capacity
	}
	return sum
}

func (node *Node) Select() bool {
	// Don't select disconnected nodes.
	if node.NumHops == -1 {
		return false
	}

	return node.NumHops > 2 &&
		node.CumulativeFee > 200 &&
		node.Capacity() > 10e6 &&
		node.NumChan() > 20
}

type ByUtility []*Node

func (nn ByUtility) Len() int        { return len(nn) }
func (nn ByUtility) Swap(ii, jj int) { nn[ii], nn[jj] = nn[jj], nn[ii] }
func (nn ByUtility) Less(ii, jj int) bool {
	if nn[ii].NumHops < nn[jj].NumHops {
		return true
	} else if nn[ii].NumHops > nn[jj].NumHops {
		return false
	} else {
		if nn[ii].CumulativeFee < nn[jj].CumulativeFee {
			return true
		} else if nn[ii].CumulativeFee > nn[jj].CumulativeFee {
			return false
		} else {
			if nn[ii].Capacity() < nn[jj].Capacity() {
				return true
			} else if nn[ii].Capacity() > nn[jj].Capacity() {
				return false
			} else {
				return nn[ii].NumChan() < nn[jj].NumChan()
			}
		}
	}
}

const xfersize = 1000 * 1000 // 1e6 sat = $80

func (node *Node) Propagate(hops int, fee float64) {
	if node.NumHops == -1 {
		// First time we've seen this node.
		node.NumHops = hops
		node.CumulativeFee = fee
	} else {
		// We've seen this node before, is this a cheaper path?
		if fee < node.CumulativeFee {
			// This is a cheaper path
			node.NumHops = hops
			node.CumulativeFee = fee
			// Fall through and repropogate it.
		} else {
			// This path is not cheaper, bail.
			return
		}
	}

	// Recursively call on our peers.
	nexthop := hops + 1
	for ndx, peer := range node.Peers {
		policy := node.Policy[ndx]
		edge := node.Edges[ndx]

		if policy != nil {
			// Does this edge have enough capacity?
			if edge.Capacity > xfersize {
				// Compute the fee to use this channel.
				nextfee := fee +
					(float64(policy.FeeBaseMsat) / 1e3) +
					(xfersize * (float64(policy.FeeRateMilliMsat) / 1e6))
				peer.Propagate(nexthop, nextfee)
			}
		}
	}
}

func farSide(cfg *config, client lnrpc.LightningClient, ctx context.Context) {
	rsp, err := client.DescribeGraph(ctx, &lnrpc.ChannelGraphRequest{})
	if err != nil {
		fmt.Println("Cannot describe graph from node:", err)
		return
	}

	nodes := map[string]*Node{}
	for _, nn := range rsp.Nodes {
		nodes[nn.PubKey] = NewNode(nn)
	}

	edges := map[uint64]*Edge{}
	for _, ee := range rsp.Edges {
		edge := NewEdge(ee)
		edges[ee.ChannelId] = edge
		// Wired so "Sender" policy is seen.
		nodes[ee.Node1Pub].AddEdge(edge, ee.Node1Policy, nodes[ee.Node2Pub])
		nodes[ee.Node2Pub].AddEdge(edge, ee.Node2Policy, nodes[ee.Node1Pub])
	}

	bonsai := nodes["02a5fa844d310f582d209fe649352b225440b8a54e77361f229bb92ee263c87e6f"]
	bonsai.Propagate(0, 0)

	selected := []*Node{}
	for _, vv := range nodes {
		if vv.Select() {
			selected = append(selected, vv)
		}
	}
	sort.Sort(ByUtility(selected))
	for _, nn := range selected {
		fmt.Printf("%s %9.2f %2d [%4.2f, %3d]\n",
			nn.LightningNode.PubKey,
			nn.CumulativeFee,
			nn.NumHops,
			math.Log10(float64(nn.Capacity())),
			nn.NumChan())
	}
}
