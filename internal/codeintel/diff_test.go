package codeintel

import "testing"

func findFile(files []FileDiff, path string) (FileDiff, bool) {
	for _, f := range files {
		if f.Path == path {
			return f, true
		}
	}
	return FileDiff{}, false
}

func TestParseUnifiedDiffModified(t *testing.T) {
	out := `diff --git a/foo.go b/foo.go
index 111..222 100644
--- a/foo.go
+++ b/foo.go
@@ -10,3 +12,5 @@ func A() {
 ctx
+nuevo
+nuevo2
 ctx
`
	files := ParseUnifiedDiff(out)
	f, ok := findFile(files, "foo.go")
	if !ok || f.ChangeType != ChangeModified {
		t.Fatalf("foo.go debería ser modified, got %+v ok=%v", f, ok)
	}
	if len(f.NewRanges) != 1 || f.NewRanges[0].Start != 12 || f.NewRanges[0].End != 16 {
		t.Errorf("rango nuevo esperado 12-16, got %+v", f.NewRanges)
	}
}

func TestParseUnifiedDiffAddedAndDeleted(t *testing.T) {
	out := `diff --git a/new.ts b/new.ts
new file mode 100644
--- /dev/null
+++ b/new.ts
@@ -0,0 +1,3 @@
+a
+b
+c
diff --git a/old.py b/old.py
deleted file mode 100644
--- a/old.py
+++ /dev/null
@@ -1,2 +0,0 @@
-x
-y
`
	files := ParseUnifiedDiff(out)
	if f, ok := findFile(files, "new.ts"); !ok || f.ChangeType != ChangeAdded {
		t.Errorf("new.ts debería ser added, got %+v ok=%v", f, ok)
	}
	// old.py es borrado puro: sin rango nuevo (count 0 descartado).
	if f, ok := findFile(files, "old.py"); !ok || f.ChangeType != ChangeDeleted || len(f.NewRanges) != 0 {
		t.Errorf("old.py debería ser deleted sin rangos nuevos, got %+v ok=%v", f, ok)
	}
}

func TestParseUnifiedDiffRename(t *testing.T) {
	out := `diff --git a/old/name.go b/new/name.go
similarity index 95%
rename from old/name.go
rename to new/name.go
--- a/old/name.go
+++ b/new/name.go
@@ -5,2 +5,3 @@
 ctx
+add
`
	files := ParseUnifiedDiff(out)
	f, ok := findFile(files, "new/name.go")
	if !ok || f.ChangeType != ChangeRenamed || f.OldPath != "old/name.go" {
		t.Errorf("rename mal parseado, got %+v ok=%v", f, ok)
	}
}

func TestParseUnifiedDiffBinary(t *testing.T) {
	out := `diff --git a/img.png b/img.png
index 111..222 100644
Binary files a/img.png and b/img.png differ
`
	files := ParseUnifiedDiff(out)
	f, ok := findFile(files, "img.png")
	if !ok || !f.Binary || len(f.NewRanges) != 0 {
		t.Errorf("binario debería marcarse sin rangos, got %+v ok=%v", f, ok)
	}
}

func TestSymbolsInRanges(t *testing.T) {
	syms := []Symbol{
		{Name: "A", StartLine: 1, EndLine: 5},
		{Name: "B", StartLine: 10, EndLine: 20},
		{Name: "C", StartLine: 30, EndLine: 40},
	}
	// Rango 12-14 cae dentro de B; borde 5-5 toca el final de A.
	got := SymbolsInRanges(syms, []LineRange{{Start: 12, End: 14}, {Start: 5, End: 5}})
	if len(got) != 2 {
		t.Fatalf("esperaba 2 símbolos (A por borde, B por dentro), got %+v", got)
	}
	names := map[string]bool{}
	for _, s := range got {
		names[s.Name] = true
	}
	if !names["A"] || !names["B"] || names["C"] {
		t.Errorf("esperaba A y B, no C; got %+v", names)
	}
}
