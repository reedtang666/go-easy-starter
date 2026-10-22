package user

import "testing"

func TestUsernameAndPasswordBoundaries(t *testing.T) {
	if v, e := NormalizeUsername("  Test_User "); e != nil || v != "test_user" {
		t.Fatal(v, e)
	}
	for _, v := range []string{"a", "space user", "用户名", "bad!name"} {
		if _, e := NormalizeUsername(v); e == nil {
			t.Fatal("accepted", v)
		}
	}
	if ValidatePassword("short") == nil {
		t.Fatal("weak input accepted")
	}
	if e := ValidatePassword("testPassword123"); e != nil {
		t.Fatal(e)
	}
}
