package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

type BroadcastNode struct {
	node *maelstrom.Node

	store      map[int]struct{}
	storeMutex sync.RWMutex

	gossipState map[int]map[string]struct{} // Message -> [nodes it hasn't reached yet]
	gossipMutex sync.Mutex
}

func NewBroadcastNode() *BroadcastNode {
	bn := BroadcastNode{
		node:        maelstrom.NewNode(),
		gossipState: make(map[int]map[string]struct{}),
		store:       make(map[int]struct{}),
	}
	bn.node.Handle("broadcast", bn.broadcastHandler)
	bn.node.Handle("read", bn.readHandler)
	bn.node.Handle("topology", bn.topologyHandler)
	return &bn
}

func (bn *BroadcastNode) broadcastHandler(msg maelstrom.Message) error {
	var body map[string]any
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	m, ok := body["message"].(float64)
	if !ok {
		return fmt.Errorf("can't parse input message")
	}
	resp := map[string]any{"type": "broadcast_ok"}
	bn.storeMutex.RLock()
	_, ok = bn.store[int(m)]
	bn.storeMutex.RUnlock()
	if ok {
		return bn.node.Reply(msg, resp) // no work to do
	}
	bn.storeMutex.Lock()
	bn.store[int(m)] = struct{}{}
	bn.storeMutex.Unlock()
	bn.addToGossipState(int(m))
	return bn.node.Reply(msg, resp)
}

func (bn *BroadcastNode) readHandler(msg maelstrom.Message) error {
	var body map[string]any
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	var out []int
	for k, _ := range bn.store {
		out = append(out, k)
	}
	resp := map[string]any{"type": "read_ok", "messages": out}
	return bn.node.Reply(msg, resp)
}

func (bn *BroadcastNode) topologyHandler(msg maelstrom.Message) error {
	return bn.node.Reply(msg, map[string]any{"type": "topology_ok"})
}

func (bn *BroadcastNode) addToGossipState(m int) {
	remaining := make(map[string]struct{})
	for _, nid := range bn.node.NodeIDs() {
		if nid == bn.node.ID() {
			continue
		}
		remaining[nid] = struct{}{}
	}
	bn.gossipMutex.Lock()
	defer bn.gossipMutex.Unlock()
	bn.gossipState[m] = remaining
}

func (bn *BroadcastNode) gossip(ctx context.Context, tick *time.Ticker) {
	for {
		select {
		// TODO: general concurrency sanitization re: ctx.Done(), maybe other things
		// TODO: should this be channel-based, rather than sync-based?
		case <-tick.C:
			// TODO: Do we really wanna try to send ALL gossip state every tick?
			for m, nodes := range bn.gossipState {
				for nid := range nodes {
					_ = bn.node.RPC(nid, map[string]any{"type": "broadcast", "message": m}, bn.gossipResponseHandler(m))
				}
			}
		}
	}
}

func (bn *BroadcastNode) gossipResponseHandler(m int) maelstrom.HandlerFunc {
	return func(msg maelstrom.Message) error {
		if msg.Type() == "broadcast_ok" {
			bn.gossipMutex.Lock()
			delete(bn.gossipState[m], msg.Src)
			bn.gossipMutex.Unlock()
		}
		if len(bn.gossipState[m]) == 0 {
			bn.gossipMutex.Lock()
			delete(bn.gossipState, m)
			bn.gossipMutex.Unlock()
		}
		return nil
	}
}

func main() {
	n := NewBroadcastNode()
	go n.gossip(context.Background(), time.NewTicker(500*time.Millisecond))
	if err := n.node.Run(); err != nil {
		log.Fatal(err)
	}
}
