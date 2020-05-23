package mcts

import (
	"log"
	"math"
	"sync"
	//	"os"

	"github.com/pkg/errors"
)

type Environment interface {
	NumActions() int
	Act(int)
	Reward() float64
}

type node struct {
	children []*node
	value    float64
	n        float64
}

type MCTS struct {
	pool sync.Pool
	root *node
}

func NewMCTS() *MCTS {
	algo := &MCTS{}
	algo.pool = sync.Pool{
		New: func() interface{} {
			return &node{}
		},
	}
	return algo
}

func (algo *MCTS) BestAction() int {
	if true {
		ns := make([]int, 0, len(algo.root.children))
		vs := make([]float64, 0, len(algo.root.children))
		for _, child := range algo.root.children {
			ns = append(ns, int(child.n))
			vs = append(vs, child.value/child.n)
		}
		//log.Printf("%+v %+v", vs, ns)
		//os.Exit(0)
	}

	maxA := 0
	maxV := algo.root.children[maxA].value / algo.root.children[maxA].n
	for a := 1; a < len(algo.root.children); a++ {
		value := algo.root.children[a].value / algo.root.children[a].n
		if value > maxV {
			maxA = a
			maxV = value
		}
	}
	return maxA
}

func (algo *MCTS) NewRoot() {
	algo.root = algo.getNode()
}

func (algo *MCTS) Rollout(env Environment, exploration float64) {
	type nodeValue struct {
		node   *node
		reward float64
	}

	//nowAct := 0

	traversal := make([]nodeValue, 0)
	curNode := algo.root
	for {
		curNode.n += 1
		reward := env.Reward()
		nv := nodeValue{node: curNode, reward: reward}
		traversal = append(traversal, nv)

		algo.setChildren(env, curNode)
		if len(curNode.children) == 0 {
			break
		}
		action := selectAction(curNode, exploration)

		env.Act(action)
		curNode = curNode.children[action]

		//nowAct = action
	}

	var accReward float64
	for i := len(traversal) - 1; i >= 0; i-- {
		node := traversal[i].node
		reward := traversal[i].reward

		accReward += reward
		node.value += accReward
	}
	//log.Printf("trrtr %+v %d c0: %+v, chiold2: %+v", traversal, nowAct, algo.root.children[0], algo.root.children[2])
}

func (algo *MCTS) ReleaseMem() {
	algo.releaseMem(algo.root)
}

func (algo *MCTS) releaseMem(n *node) {
	for _, child := range n.children {
		algo.releaseMem(child)
	}
	algo.pool.Put(n)
}

func selectAction(n *node, exploration float64) int {
	maxA := 0
	child := n.children[maxA]
	if child.n == 0 {
		return maxA
	}
	maxV := child.value/child.n + exploration*math.Sqrt(math.Log(n.n)/child.n)

	for a := 1; a < len(n.children); a++ {
		child := n.children[a]
		if child.n == 0 {
			return a
		}
		childValue := child.value/child.n + exploration*math.Sqrt(math.Log(n.n)/child.n)

		if childValue > maxV {
			maxA = a
			maxV = childValue
		}
	}

	return maxA
}

func (algo *MCTS) setChildren(env Environment, n *node) {
	if len(n.children) > 0 {
		return
	}
	for a := 0; a < env.NumActions(); a++ {
		child := algo.getNode()
		n.children = append(n.children, child)
	}
}

func (algo *MCTS) getNode() *node {
	n := algo.pool.Get().(*node)
	n.children = n.children[:0]
	n.value = 0
	n.n = 0
	return n
}

func dummy() error {
	log.Printf("qq")
	return errors.New("sdfj")
}
