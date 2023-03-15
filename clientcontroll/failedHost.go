package clientcontroll

import "gitee.com/dark.H/gs"

type Set[A comparable] gs.List[A]

func (c Set[A]) Add(one A) Set[A] {
	if !c.In(one) {
		c = append(c, one)
	}
	return c
}

func (c Set[A]) In(one A) bool {
	return gs.List[A](c).In(one, func(compareItem, item A) bool {
		return compareItem == item
	})
}

func (c Set[A]) Del(one A) Set[A] {
	c2 := Set[A]{}
	c.Every(func(i A) {
		if i == one {
			return
		}
		c2 = append(c2, i)
	})
	return c2
}

func (c Set[A]) Count() int {
	return len(c)
}

func (c Set[A]) List() gs.List[A] {
	return gs.List[A](c)
}

func (c Set[A]) Every(ef func(one A)) Set[A] {
	gs.List[A](c).Every(func(no int, i A) {
		ef(i)
	})
	return c
}
