package starform

type TestCacheBase struct{}

func (*TestCacheBase) private() {}

const RunDataLocalKey = runDataLocalKey

type RunData = runData
