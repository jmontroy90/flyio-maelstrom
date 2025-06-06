# BROADCAST

- 3a: Single Node
  - Easy. Single node, so it gets everything.
  - No need to do anything other than handle the messages.
  - Topology ignored.
- 3b: Multi-Node, No Nemesis
  - Multi-node means broadcast.
  - We try to broadcast on every `broadcast` RPC received.
  - `n.NodeIDs` is populated on the `init` RPC, and every node always has a list of all nodes.
- 3c: Multi-Node, Partition Nemesis
    - Lots learned here!
    - In Maelstrom - and distributed systems more generally - you can't rely on timeout error signals when you try to broadcast a message; sometimes, you send the message and it's just...gone. So 
      the trick here was to keep trying to gossip out messages per node until you receive a confirming response from your target node.
    - Had some really annoying locking / channel bugs that I've tried to document in the code.
    - I did two iterations on this:
      1) Giant guarded gossip state, gossiped on a poll interval. This is "shared state + mutex", where I kept state of which messages had confirmed receipt on which nodes, and unconfirmed were 
         gossiped until confirmation. This worked, but I was dissatisfied with the size of the critical section. It was a BIG lock on state that you're essentially reading, writing, and ranging 
         over all simultaneously. I could've redone the state storage, and stored a list of messages needed to be broadcast per node, and then have a map of locks, one per node, to keep concurrent 
         reads and writes more isolated and performant.
      2) Channels + event loop with "self-retrying" gossip requests - each gossip request targets a single node, and manages its own `retryAt` and `numRetries` state. The gossip requests are sent 
         to a gossip thread that tries to send the requests out and confirm receipt. The requests are then sent to a retry loop, which checks for gossip requests that should be retried at some 
         time. It does this on an interval. I still need shared state for the node's message state + state indicating which nodes have confirmed receipt, since that's done in the callback. I 
         suspect if I used `SyncRPC`, I might be able to eliminate shared state for gossip progress, but then I'd have blocking calls on the gossip loop. Overall, I was glad for the exercise, but I'm 
         not sure this is at all better. It's less synchronization and smaller critical paths, but it's more Goroutines. 