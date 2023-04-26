package wizard

type FakeStorage struct{}

func (FakeStorage) GetCurrentState(int64, Wizard) error { return nil }
func (FakeStorage) SaveState(int64, Wizard) error       { return nil }
func (FakeStorage) DeleteState(int64) error             { return nil }
func (FakeStorage) Close() error                        { return nil }
