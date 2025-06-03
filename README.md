# Gossip Glomers

These are Golang solutions to Fly.io's [Gossip Glomers](https://fly.io/dist-sys/) distributed systems challenges, based on [Maelstrom](https://github.com/jepsen-io/maelstrom/tree/main).

## Getting Started

First step is to set up Maelstrom itself. Start with [Fly.io's first part](https://fly.io/dist-sys/1/) - cmd+F "Installing Maelstrom"; there are some Java prereqs you need first.

You also need a copy of Maelstrom to run locally. This repo setup expects it at `./maelstrom`. Fly.io expects v0.2.3, which you can get [here](https://github.com/jepsen-io/maelstrom/releases/tag/v0.2.3).

If you like, you can run this from the top level of this directory:
```bash
wget https://github.com/jepsen-io/maelstrom/releases/download/v0.2.3/maelstrom.tar.bz2 -O ./maelstrom.tar.bz2
tar -xjf ./maelstrom.tar.bz2 -C ./
rm ./maelstrom.tar.bz2
```

### Running Challenges

This repo uses [Magefile](https://magefile.org/) for make-like commands.

```sh
> mage
Targets:
  build            builds Go binary for {name}
  ls               lists the available challenge:part combos to run.
  run:challenge    runs the given {challenge}
  run:part         runs the given {challenge}, {part}.
```

Run a given challenge:
```bash
> mage run:challenge echo
```

Run a given part:
```bash
> mage run:part broadcast 3a
```

List all runnable challenges / parts (this will grow):

```bash
> mage ls
    - echo [-]
    - unique-ids [-]
    - broadcast [3a,3b,3c]
```
