package manifest

// Manifest is the typed view of a schema-valid manifest. Only fields the server
// reasons about are typed; everything else stays in the raw JSON.
type Manifest struct {
	Schema      string     `json:"schema"`
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Kind        string     `json:"kind"`
	Version     string     `json:"version"`
	Description string     `json:"description"`
	Taxonomy    Taxonomy   `json:"taxonomy"`
	Musical     *Musical   `json:"musical"`
	Module      *Module    `json:"module"`
	Deps        []Dep      `json:"dependencies"`
	Artifacts   []Artifact `json:"artifacts"`
	Samples     []Sample   `json:"samples"`
}

type Taxonomy struct {
	Genres struct {
		Primary   string   `json:"primary"`
		Secondary []string `json:"secondary"`
	} `json:"genres"`
	Instruments []string `json:"instruments"`
	Moods       []string `json:"moods"`
	UseCases    []string `json:"useCases"`
	Energy      *float64 `json:"energy"`
}

type Musical struct {
	BPM *struct {
		Value int   `json:"value"`
		Range []int `json:"range"`
	} `json:"bpm"`
}

type Module struct {
	ModuleClass  string `json:"moduleClass"`
	Capabilities struct {
		Accepts  []string `json:"accepts"`
		Produces []string `json:"produces"`
	} `json:"capabilities"`
	Ports []struct {
		Direction string `json:"direction"`
		Signal    string `json:"signal"`
	} `json:"ports"`
	Categories []string `json:"categories"`
}

type Dep struct {
	ID    string `json:"id"`
	Range string `json:"range"`
}

type Artifact struct {
	Kind        string `json:"kind"`
	Path        string `json:"path"`
	Digest      string `json:"digest"`
	Size        int64  `json:"size"`
	ContentType string `json:"contentType"`
}

type Sample struct {
	ID   string `json:"id"`
	File string `json:"file"`
}

// TaxonomyTerm is one row for package_taxonomy (search index, spec §7).
type TaxonomyTerm struct {
	Domain string
	Slug   string
	Weight float64
}

// Terms lists the controlled-vocabulary terms of a manifest with search weights.
func (m *Manifest) Terms() []TaxonomyTerm {
	seen := map[string]bool{}
	var out []TaxonomyTerm
	add := func(domain, slug string, w float64) {
		k := domain + ":" + slug
		if slug == "" || seen[k] {
			return
		}
		seen[k] = true
		out = append(out, TaxonomyTerm{domain, slug, w})
	}
	add("genre", m.Taxonomy.Genres.Primary, 1)
	for _, g := range m.Taxonomy.Genres.Secondary {
		add("genre", g, 0.5)
	}
	for i, s := range m.Taxonomy.Instruments {
		w := 0.5
		if i == 0 {
			w = 1 // spec §7.3: the first instrument is the primary one
		}
		add("instrument", s, w)
	}
	for _, s := range m.Taxonomy.Moods {
		add("mood", s, 1)
	}
	for _, s := range m.Taxonomy.UseCases {
		add("use-case", s, 1)
	}
	if m.Module != nil {
		for _, s := range m.Module.Categories {
			add("module-category", s, 1)
		}
	}
	return out
}
