package lua

import (
	"fmt"
	"testing"
	"time"
)

func TestStrftime(t *testing.T) {
	type testCase struct {
		T        time.Time
		Fmt      string
		Expected string
	}

	t1 := time.Date(2016, time.February, 3, 13, 23, 45, 123, time.FixedZone("Plus2", 60*60*2))
	t2 := time.Date(1945, time.September, 6, 7, 35, 4, 989, time.FixedZone("Minus5", 60*60*-5))

	cases := []testCase{
		{t1, "foo%nbar%tbaz 100%% cool", "foo\nbar\tbaz 100% cool"},

		{t1, "%Y %y", "2016 16"},
		{t1, "%G %g", "2016 16"},
		{t1, "%b %B", "Feb February"},
		{t1, "%m %-m", "02 2"},
		{t1, "%V", "5"},
		{t1, "%w", "3"},
		{t1, "%j", "034"},
		{t1, "%d %-d %e", "03 3  3"},
		{t1, "%a %A", "Wed Wednesday"},
		{t1, "%H %I %l", "13 01 1"},
		{t1, "%M", "23"},
		{t1, "%S", "45"},
		{t1, "%c", "03 Feb 16 13:23 Plus2"},
		{t1, "%D %x", "02/03/16 02/03/16"},
		{t1, "%F", "2016-02-03"},
		{t1, "%r", "01:23:45 PM"},
		{t1, "%R %T %X", "13:23 13:23:45 13:23:45"},
		{t1, "%p %P", "PM pm"},
		{t1, "%z %Z", "+0200 Plus2"},

		{t2, "%Y %y", "1945 45"},
		{t2, "%G %g", "1945 45"},
		{t2, "%b %B", "Sep September"},
		{t2, "%m %-m", "09 9"},
		{t2, "%V", "36"},
		{t2, "%w", "4"},
		{t2, "%j", "249"},
		{t2, "%d %-d %e", "06 6  6"},
		{t2, "%a %A", "Thu Thursday"},
		{t2, "%H %I %l", "07 07 7"},
		{t2, "%M", "35"},
		{t2, "%S", "04"},
		{t2, "%c", "06 Sep 45 07:35 Minus5"},
		{t2, "%D %x", "09/06/45 09/06/45"},
		{t2, "%F", "1945-09-06"},
		{t2, "%r", "07:35:04 AM"},
		{t2, "%R %T %X", "07:35 07:35:04 07:35:04"},
		{t2, "%p %P", "AM am"},
		{t2, "%z %Z", "-0500 Minus5"},

		{t1, "not real flags: %-Q %_J %^^ %-", "not real flags: %-Q %_J %^^ %-"},
		{t1, "end in flag: %", "end in flag: %"},
	}
	for i, c := range cases {
		t.Run(fmt.Sprintf("Case %d (\"%s\")", i, c.Fmt), func(t *testing.T) {
			actual := strftime(c.T, c.Fmt)
			if actual != c.Expected {
				t.Errorf("bad strftime: expected \"%s\" but got \"%s\"", c.Expected, actual)
			}
		})
	}
}
