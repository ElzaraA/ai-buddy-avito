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

// R2-8: индекс сообщает, что ответ есть, но для другой аудитории (без раскрытия фрагмента).
func TestRestricted(t *testing.T) {
	idx := kb()
	if !idx.Restricted("наставник проводит", "backend", "intern") {
		t.Fatal("mentors-only chunk must be reported as restricted for intern")
	}
	if idx.Restricted("наставник проводит", "backend", "buddy") {
		t.Fatal("mentor sees the chunk: not restricted")
	}
	if idx.Restricted("наставник проводит", "frontend", "intern") {
		t.Fatal("other team's chunk is not restricted, it is absent")
	}
	if idx.Restricted("секретный текст", "backend", "intern") || idx.Restricted("Сколько платят?", "backend", "intern") {
		t.Fatal("chunk without audience or no match is not restricted")
	}
}

// AI-2: одно общее слово не делает фрагмент ответом. Короткий запрос — совпасть должны все слова,
// длинный — по весу IDF: частые слова («стажёр», «заявка») не тянут нерелевантный ответ.
func TestCoverageRequiresTopicWords(t *testing.T) {
	idx := kb()
	for _, q := range []string{"Какая зарплата у стажёра?", "Удали все заявки", "Как получить апрув?", "Какая ипотека и зарплата у стажёра?"} {
		if hits := idx.Search(q, "backend", "intern", 3); len(hits) != 0 {
			t.Fatalf("%q must not match, got %s", q, hits[0].Chunk.ID)
		}
	}
	if hits := idx.Search("Как оформить отгул?", "backend", "intern", 3); len(hits) == 0 || hits[0].Chunk.Title != "Отгул" {
		t.Fatalf("two topic words must match: %+v", hits)
	}
	if hits := idx.Search("Как в первую неделю получить доступы к Jira?", "backend", "intern", 3); len(hits) == 0 || hits[0].Chunk.Title != "Чек-лист стажёра" {
		t.Fatalf("long query with weighted coverage: %+v", hits)
	}
}

// AI-5: формы слов, опечатки и транслитерация находят тот же фрагмент.
func TestStemmingAndSynonyms(t *testing.T) {
	md := `# Окружение
<!-- stage: week1 team: backend audience: intern -->
Локальная база PostgreSQL поднимается через docker compose. Установить Go 1.22.

# Встречи
<!-- stage: week1 team: backend audience: intern -->
Стендап каждый день в 11:00. Frontend-стек: React. Мержить после апрува.`
	idx := NewIndex(ChunkMarkdown("k.md", md))
	cases := map[string]string{
		"Как поднять локальную базу?": "Окружение",
		"Какую версию Go ставить?":    "Окружение",
		"Во сколько стэндап?":         "Встречи",
		"Какой стек у фронтенда?":     "Встречи",
		"Когда можно мерджить?":       "Встречи",
	}
	for q, want := range cases {
		if hits := idx.Search(q, "backend", "intern", 3); len(hits) == 0 || hits[0].Chunk.Title != want {
			t.Fatalf("%q: want %s, got %+v", q, want, hits)
		}
	}
	for a, b := range map[string]string{"базу": "база", "выдаёт": "выдают", "доступы": "доступ"} {
		if !termMatch(a, b) {
			t.Fatalf("%s ~ %s", a, b)
		}
	}
	if termMatch("код", "кот") || stem("да") != "да" {
		t.Fatal("short words must not be stemmed into collisions")
	}
	if Meaningful("??? 2026") || !Meaningful("Где парковка?") {
		t.Fatal("Meaningful")
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
