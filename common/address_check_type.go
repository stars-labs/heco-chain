package common

const (
	CheckNone AddressCheckType = iota
	CheckFrom
	CheckTo
	CheckBothInAny
)

type AddressCheckType int
