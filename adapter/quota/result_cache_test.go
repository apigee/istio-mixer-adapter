package quota

import "testing"

func TestResultCache(t *testing.T) {
	results := ResultCache{
		size: 2,
	}

	tests := []struct {
		add       string
		exists    []string
		notExists []string
	}{
		{"test1", []string{"test1"}, []string{""}},
		{"test2", []string{"test1", "test2"}, []string{""}},
		{"test3", []string{"test2", "test3"}, []string{"test1"}},
		{"test1", []string{"test1", "test3"}, []string{"test2"}},
		{"test2", []string{"test1", "test2"}, []string{"test3"}},
	}

	for i, test := range tests {
		results.Add(test.add, &Result{})
		for _, id := range test.exists {
			if results.Get(id) == nil {
				t.Errorf("test[%d] %s value %s should exist", i, test.add, id)
			}
		}
		for _, id := range test.notExists {
			if results.Get(id) != nil {
				t.Errorf("test[%d] %s value %s should not exist", i, test.add, id)
			}
		}
	}
}
