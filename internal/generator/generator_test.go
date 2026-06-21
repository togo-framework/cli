package generator

import "testing"

func TestCaseHelpers(t *testing.T) {
	cases := []struct{ in, pascal, camel, snake, kebab string }{
		{"blog post", "BlogPost", "blogPost", "blog_post", "blog-post"},
		{"post_id", "PostID", "postID", "post_id", "post-id"},
		{"user_name", "UserName", "userName", "user_name", "user-name"},
		{"Comment", "Comment", "comment", "comment", "comment"},
	}
	for _, c := range cases {
		if got := Pascal(c.in); got != c.pascal {
			t.Errorf("Pascal(%q) = %q, want %q", c.in, got, c.pascal)
		}
		if got := Snake(c.in); got != c.snake {
			t.Errorf("Snake(%q) = %q, want %q", c.in, got, c.snake)
		}
		if got := Kebab(c.in); got != c.kebab {
			t.Errorf("Kebab(%q) = %q, want %q", c.in, got, c.kebab)
		}
	}
}

func TestTableName(t *testing.T) {
	for in, want := range map[string]string{"Post": "posts", "Category": "categories", "Person": "people"} {
		if got := TableName(in); got != want {
			t.Errorf("TableName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseFields(t *testing.T) {
	fields, err := ParseFields([]string{"title:string", "body:text?", "views:int", "tag:string:nullable"})
	if err != nil {
		t.Fatal(err)
	}
	if len(fields) != 4 {
		t.Fatalf("got %d fields, want 4", len(fields))
	}
	if fields[0].GQL != "String!" {
		t.Errorf("non-null gql = %q, want String!", fields[0].GQL)
	}
	if !fields[1].Null || fields[1].GQL != "String" {
		t.Errorf("nullable field not parsed: %+v", fields[1])
	}
	if fields[2].Go != "int64" || fields[2].PG != "bigint" {
		t.Errorf("int mapping wrong: %+v", fields[2])
	}
	if !fields[3].Null {
		t.Errorf(":nullable modifier not applied: %+v", fields[3])
	}

	if _, err := ParseFields([]string{"bogus"}); err == nil {
		t.Error("expected error for field without type")
	}
	if _, err := ParseFields([]string{"x:notatype"}); err == nil {
		t.Error("expected error for unknown type")
	}
}
