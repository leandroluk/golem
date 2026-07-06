package must

func Value[T any](val T, err error) T {
	if err != nil {
		panic(err)
	}
	return val
}

func Exec(err error) {
	if err != nil {
		panic(err)
	}
}

func Recover(err *error) {
	if r := recover(); r != nil {
		if e, ok := r.(error); ok {
			*err = e
		} else {
			panic(r)
		}
	}
}
