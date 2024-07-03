package set

type SetString struct {
	m map[string]bool
}

func NewSetString(strs ...string) *SetString {
	ss := &SetString{
		m: make(map[string]bool),
	}

	for _, str := range strs {
		ss.Add(str)
	}

	return ss
}
func (ss *SetString) Add(str string) {
	ss.m[str] = true
}

func (ss *SetString) Del(str string) {
	delete(ss.m, str)
}

func (ss *SetString) Find(str string) bool {
	_, ok := ss.m[str]
	return ok
}

func (ss *SetString) Get(str string) bool {
	return ss.m[str]
}
