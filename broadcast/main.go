package main

import (
	"encoding/json"
	"fmt"
	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
	"log"
)

type BroadcastNode struct {
	Node           *maelstrom.Node
	BroadcastStore map[int][]string // Message -> [nodes it hasn't reached yet]
	Store          map[int]struct{}
}

func NewBroadcastNode() *BroadcastNode {
	bn := BroadcastNode{
		Node:           maelstrom.NewNode(),
		BroadcastStore: make(map[int][]string),
		Store:          make(map[int]struct{}),
	}
	bn.Node.Handle("broadcast", bn.broadcastHandler)
	bn.Node.Handle("read", bn.readHandler)
	bn.Node.Handle("topology", bn.topologyHandler)
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
	resp := map[string]any{
		"type": "broadcast_ok",
	}
	if _, ok := bn.Store[int(m)]; ok {
		return bn.Node.Reply(msg, resp) // no work to do
	}
	bn.Store[int(m)] = struct{}{}
	return bn.Node.Reply(msg, resp)
}

func (bn *BroadcastNode) readHandler(msg maelstrom.Message) error {
	var body map[string]any
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}

	var out []int
	for k, _ := range bn.Store {
		out = append(out, k)
	}
	resp := map[string]any{
		"type":     "read_ok",
		"messages": out,
	}
	return bn.Node.Reply(msg, resp)
}

func (bn *BroadcastNode) topologyHandler(msg maelstrom.Message) error {
	return bn.Node.Reply(msg, map[string]any{"type": "topology_ok"})
}

func main() {
	n := NewBroadcastNode()
	if err := n.Node.Run(); err != nil {
		log.Fatal(err)
	}
}
