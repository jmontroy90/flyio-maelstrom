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
	node            *maelstrom.Node
	currentTopology map[string][]string
	store           *sync.Map
	gossipCh        chan gossipRequest
	gossipState     *sync.Map
	retryCh         chan gossipRequest
	retryInterval   *time.Ticker
}

type gossipRequest struct {
	msg        int
	node       string
	retryAt    time.Time
	numRetries int
}

type nodeMsg struct {
	msg  int
	node string
}

func NewBroadcastNode() *BroadcastNode {
	bn := BroadcastNode{
		node:        maelstrom.NewNode(),
		gossipState: &sync.Map{},
		store:       &sync.Map{},
		// TODO: Need to figure out how to calibrate the size of both of these channels; maybe a Stats() goroutine?
		gossipCh: make(chan gossipRequest, 5000),
		retryCh:  make(chan gossipRequest, 5000),
		// TODO: more configuration on retry intervals, etc.
		retryInterval: time.NewTicker(500 * time.Millisecond),
	}
	bn.node.Handle("broadcast", bn.broadcastHandler)
	bn.node.Handle("read", bn.readHandler)
	bn.node.Handle("topology", bn.topologyHandler)
	return &bn
}

// TODO: lots of little helpers and code refactoring could help this all a lot
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
	_, ok = bn.store.Load(int(m))
	if ok {
		// DO NOT LOCK THIS CALL ACCIDENTALLY!
		return bn.node.Reply(msg, resp) // no work to do
	}
	bn.store.Store(int(m), struct{}{})
	bn.sendToNeighbors(int(m))
	return bn.node.Reply(msg, resp)
}

func (bn *BroadcastNode) readHandler(msg maelstrom.Message) error {
	var body map[string]any
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	var out []int
	bn.store.Range(func(key, value any) bool {
		out = append(out, key.(int))
		return true
	})
	resp := map[string]any{"type": "read_ok", "messages": out}
	return bn.node.Reply(msg, resp)
}

// TODO: actually like use this
func (bn *BroadcastNode) topologyHandler(msg maelstrom.Message) error {
	var body map[string]any
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	topo := convertSliceMap(body["topology"])
	bn.currentTopology = topo
	return bn.node.Reply(msg, map[string]any{"type": "topology_ok"})
}

func convertSliceMap(input any) map[string][]string {
	kvs := input.(map[string]interface{})
	res := make(map[string][]string, len(kvs))
	for k, vs := range kvs {
		vsi := vs.([]interface{})
		vss := make([]string, len(vsi))
		for i, vi := range vsi {
			if vs, ok := vi.(string); ok {
				vss[i] = vs
			}
		}
		res[k] = vss
	}
	return res // edge case: empty slice
}

func (bn *BroadcastNode) sendToGossip(m int) {
	// Create individual broadcast requests for all nodes EXCEPT this one.
	for _, nid := range bn.node.NodeIDs() {
		if nid == bn.node.ID() {
			continue
		}
		bn.gossipCh <- gossipRequest{msg: m, node: nid, retryAt: time.Now()}
	}
}

func (bn *BroadcastNode) sendToNeighbors(m int) {
	// Create individual broadcast requests for all nodes EXCEPT this one.
	for _, nid := range bn.currentTopology[bn.node.ID()] {
		if nid == bn.node.ID() {
			continue
		}
		bn.gossipCh <- gossipRequest{msg: m, node: nid, retryAt: time.Now()}
	}
}

func (bn *BroadcastNode) gossipHandler(ctx context.Context) {
	for {
		select {
		case gr := <-bn.gossipCh:
			_, ok := bn.gossipState.Load(nodeMsg{msg: gr.msg, node: gr.node})
			if ok {
				continue
			}
			body := map[string]any{"type": "broadcast", "message": gr.msg}
			_ = bn.node.RPC(gr.node, body, bn.gossipResponseHandler(gr))
			gr.numRetries++ // simple linearly-growing backoff for retries
			// TODO: configurable
			backoff := 100 * time.Duration(gr.numRetries) * time.Millisecond
			if backoff > 500*time.Millisecond {
				backoff = 500 * time.Millisecond
			}
			gr.retryAt = gr.retryAt.Add(backoff)
			bn.retryCh <- gr // back into the event loop
		}
	}
}

func (bn *BroadcastNode) retryHandler(ctx context.Context) {
	for {
		select {
		case <-bn.retryInterval.C: // every interval, check what messages are ready to be processed.
			queueLen := len(bn.retryCh)
			// We run through the retry queue as it exists at the start of the poll interval, and no more;
			// since we're sending messages back into the channel, we can't just carelessly range without creating a "hot loop".
			for i := 0; i < queueLen; i++ {
				select {
				case gr := <-bn.retryCh:
					if time.Now().After(gr.retryAt) {
						bn.gossipCh <- gr
					} else {
						bn.retryCh <- gr
					}
				}
			}
		}
	}
}

func (bn *BroadcastNode) gossipResponseHandler(gr gossipRequest) maelstrom.HandlerFunc {
	return func(msg maelstrom.Message) error {
		if msg.Type() == "broadcast_ok" {
			bn.gossipState.Store(nodeMsg{msg: gr.msg, node: gr.node}, struct{}{})
		}
		return nil
	}
}

// TODO: clean Goroutine teardown via ctx
func main() {
	n := NewBroadcastNode()
	ctx := context.Background()
	go n.gossipHandler(ctx)
	go n.retryHandler(ctx)
	if err := n.node.Run(); err != nil {
		log.Fatal(err)
	}
}
