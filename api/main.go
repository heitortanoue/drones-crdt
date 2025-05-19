package main

import (
	"fmt"
	"github.com/heitortanoue/tcc/crdt"
)

func main() {
	// réplicas A e B
	a := crdt.New[string]("A")
	b := crdt.New[string]("B")

	a.Add("go")
	a.Add("crdt")

	// concurrently: B adiciona "golang" e remove "go"
	b.Add("golang")
	b.Remove("go")

	// merge bidirecional
	a.Merge(b)
	b.Merge(a)

	fmt.Println("A ->", a.Elements()) // map[crdt:{} golang:{} go:{}]
	fmt.Println("B ->", b.Elements()) // idem
	// “go” permanece porque a inserção venceu a remoção concorrente
}