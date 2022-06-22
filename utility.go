package main

func IfThenElse(_if bool, _then int, _else int) int {
	if _if {
		return _then
	} else {
		return _else
	}
}

func Max(x, y int) int {
	return IfThenElse(x > y, x, y)
}

func Min(x, y int) int {
	return IfThenElse(x > y, y, x)
}
