package rag

import "testing"

func kb() *Index {
	md := `# Отгул
<!-- stage: week1 team: backend audience: intern,buddy -->
Порядок оформления отгула: согласовать с руководителем, создать заявку в системе отсутствий.

# Чек-лист стажёра
<!-- stage: week1 team: backend audience: intern -->
Первая неделя: получить доступы к Git, Jira, Grafana.

# Для наставников
<!-- stage: week1 team: backend audience: buddy -->
Наставник проводит 1:1 раз в неделю.

# Без аудитории
<!-- stage: week1 team: backend -->
Секретный текст без audience.`
	return NewIndex(ChunkMarkdown("k.md", md))
}

func TestStopwordsDoNotMatch(t *testing.T) {
	idx := kb()
	for _, q := range []string{"Сколько платят в Авито?", "А что в Москве?", "Как получить ипотеку для сотрудников?", "2026 11 20", ""} {
		if hits := idx.Search(q, "backend", "intern", 3); len(hits) != 0 {
			t.Fatalf("%q must not match, got %s", q, hits[0].Chunk.ID)
		}
	}
}

func TestTitleIsIndexed(t *testing.T) {
	if hits := kb().Search("чек-лист стажёра", "backend", "intern", 3); len(hits) == 0 {
		t.Fatal("heading words must be searchable")
	}
}

func TestAudienceDenyByDefault(t *testing.T) {
	idx := kb()
	if hits := idx.Search("секретный текст", "backend", "intern", 3); len(hits) != 0 {
		t.Fatal("chunk without audience must be invisible")
	}
	if hits := idx.Search("наставник проводит", "backend", "intern", 3); len(hits) != 0 {
		t.Fatal("mentors-only chunk visible to intern")
	}
	if hits := idx.Search("наставник проводит", "backend", "buddy", 3); len(hits) == 0 {
		t.Fatal("mentor must see mentors-only chunk")
	}
}

func TestTermsFilter(t *testing.T) {
	got := terms("Хочу в пятницу 2026 отгул, ё-моё")
	want := map[string]bool{"пятницу": true, "отгул": true, "ё": false, "моё": true}
	for _, g := range got {
		if !want[g] {
			t.Fatalf("unexpected term %q in %v", g, got)
		}
	}
}
