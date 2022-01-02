package pointer

func Int32(i int32) *int32 {
	return &i
}

func String(s string) *string {
	return &s
}

func Bool(b bool) *bool {
	return &b
}
