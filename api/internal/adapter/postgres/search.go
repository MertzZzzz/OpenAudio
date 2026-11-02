package postgres

import (
	"context"
	"regexp"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MertzZzzz/OpenAudio/api/internal/domain"
)

// SearchRepo queries the package_search / sample_index tables maintained by
// refresh_package_search() (migration 000005).
type SearchRepo struct{ pool *pgxpool.Pool }

func NewSearchRepo(pool *pgxpool.Pool) *SearchRepo { return &SearchRepo{pool: pool} }

type args struct{ list []any }

func (a *args) add(v any) string {
	a.list = append(a.list, v)
	return "$" + strconv.Itoa(len(a.list))
}

var tokenSplit = regexp.MustCompile(`[^\p{L}\p{N}]+`)

// tsQuery turns free text into a prefix-matching AND query: "deep hous" → "deep:* & hous:*".
// Tokens are reduced to letters and digits, so the result is always valid tsquery syntax.
func tsQuery(text string) string {
	var parts []string
	for _, t := range tokenSplit.Split(strings.ToLower(text), -1) {
		if t != "" {
			parts = append(parts, t+":*")
		}
	}
	return strings.Join(parts, " & ")
}

var taxonomyFacets = map[string]string{
	domain.FacetGenre:      "genre",
	domain.FacetInstrument: "instrument",
	domain.FacetMood:       "mood",
	domain.FacetUseCase:    "use-case",
	domain.FacetCategory:   "module-category",
}

func taxonomyCond(a *args, dom string, slugs []string) string {
	return `EXISTS (SELECT 1 FROM package_taxonomy t WHERE t.release_id = s.release_id AND t.domain = ` + a.add(dom) +
		` AND t.slug::text = ANY(` + a.add(slugs) + `::text[]))`
}

// where builds the filter for package_search s, leaving out the facet named skip.
// It returns the conditions and the tsquery placeholder (if any) for ranking.
func where(a *args, q domain.SearchQuery, skip string) ([]string, string, string) {
	var conds []string
	tsArg, aliasArg := "", ""
	if tq := tsQuery(q.Text); tq != "" || len(q.AliasSlugs) > 0 {
		var alts []string
		if tq != "" {
			tsArg = a.add(tq)
			alts = append(alts, `s.document @@ to_tsquery('simple', `+tsArg+`)`)
		}
		if len(q.AliasSlugs) > 0 {
			aliasArg = a.add(q.AliasSlugs)
			alts = append(alts, `EXISTS (SELECT 1 FROM package_taxonomy t WHERE t.release_id = s.release_id AND t.slug::text = ANY(`+aliasArg+`::text[]))`)
		}
		conds = append(conds, "("+strings.Join(alts, " OR ")+")")
	}
	in := func(facet, col string, vals []string) {
		if facet != skip && len(vals) > 0 {
			conds = append(conds, col+` = ANY(`+a.add(vals)+`::text[])`)
		}
	}
	in(domain.FacetKind, "s.kind", q.Kinds)
	in(domain.FacetModuleClass, "s.module_class", q.ModuleClass)
	in(domain.FacetKey, "s.music_key", q.Keys)
	in(domain.FacetLicense, "s.license", q.Licenses)
	for facet, dom := range taxonomyFacets {
		vals := map[string][]string{
			domain.FacetGenre: q.Genres, domain.FacetInstrument: q.Instruments, domain.FacetMood: q.Moods,
			domain.FacetUseCase: q.UseCases, domain.FacetCategory: q.Categories,
		}[facet]
		if facet != skip && len(vals) > 0 {
			conds = append(conds, taxonomyCond(a, dom, vals))
		}
	}
	if q.Publisher != "" {
		conds = append(conds, `s.publisher_slug = `+a.add(q.Publisher))
	}
	if q.Commercial != nil {
		conds = append(conds, `s.commercial_use = `+a.add(*q.Commercial))
	}
	if skip != domain.FacetBPM {
		// A package matches when its BPM (or BPM range) overlaps the requested range.
		if q.BPMMin != nil {
			conds = append(conds, `COALESCE(s.bpm_max, s.bpm) >= `+a.add(*q.BPMMin))
		}
		if q.BPMMax != nil {
			conds = append(conds, `COALESCE(s.bpm_min, s.bpm) <= `+a.add(*q.BPMMax))
		}
	}
	if q.EnergyMin != nil {
		conds = append(conds, `s.energy >= `+a.add(*q.EnergyMin))
	}
	if q.EnergyMax != nil {
		conds = append(conds, `s.energy <= `+a.add(*q.EnergyMax))
	}
	return conds, tsArg, aliasArg
}

func whereSQL(conds []string) string {
	if len(conds) == 0 {
		return ""
	}
	return " WHERE " + strings.Join(conds, " AND ")
}

func (r *SearchRepo) Search(ctx context.Context, q domain.SearchQuery) ([]domain.SearchHit, int, error) {
	a := &args{}
	conds, tsArg, aliasArg := where(a, q, "")
	score := "0::float8"
	if tsArg != "" || aliasArg != "" {
		var parts []string
		if tsArg != "" {
			parts = append(parts, `ts_rank_cd(s.document, to_tsquery('simple', `+tsArg+`))`)
		}
		if aliasArg != "" {
			parts = append(parts, `CASE WHEN EXISTS (SELECT 1 FROM package_taxonomy t WHERE t.release_id = s.release_id AND t.slug::text = ANY(`+aliasArg+`::text[])) THEN 0.5 ELSE 0 END`)
		}
		score = "(" + strings.Join(parts, " + ") + ")::float8"
	}
	order := `s.published_at DESC, p.id`
	switch q.Sort {
	case domain.SearchRelevance:
		order = `score DESC, s.published_at DESC, p.id`
	case domain.SearchDownloads:
		order = `p.download_count DESC, s.published_at DESC, p.id`
	case domain.SearchName:
		order = `lower(p.name), p.id`
	}
	sql := packageColumns + `, ` + score + ` AS score, count(*) OVER () AS total` + packageFrom +
		` JOIN package_search s ON s.package_id = p.id` + whereSQL(conds) +
		` ORDER BY ` + order + ` LIMIT ` + a.add(q.Limit) + ` OFFSET ` + a.add(q.Offset)
	rows, err := r.pool.Query(ctx, sql, a.list...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []domain.SearchHit{}
	total := 0
	for rows.Next() {
		var h domain.SearchHit
		v, err := scanPackage(rows, &h.Score, &total)
		if err != nil {
			return nil, 0, err
		}
		h.PackageView = *v
		out = append(out, h)
	}
	return out, total, rows.Err()
}

// bpmBuckets are the ranges producers think in. A package is counted in every bucket its BPM
// range overlaps — the same rule the bpmMin/bpmMax filter uses, so counts match results.
const bpmBuckets = `(VALUES ('0-79', 0, 79), ('80-99', 80, 99), ('100-119', 100, 119), ('120-129', 120, 129),
  ('130-139', 130, 139), ('140-159', 140, 159), ('160-179', 160, 179), ('180-400', 180, 400)) AS b(v, lo, hi)`

func (r *SearchRepo) Facets(ctx context.Context, q domain.SearchQuery) (domain.Facets, error) {
	out := domain.Facets{}
	count := func(facet, sql string, a *args) error {
		rows, err := r.pool.Query(ctx, sql, a.list...)
		if err != nil {
			return err
		}
		defer rows.Close()
		buckets := []domain.FacetBucket{}
		for rows.Next() {
			var b domain.FacetBucket
			if err := rows.Scan(&b.Value, &b.Count); err != nil {
				return err
			}
			buckets = append(buckets, b)
		}
		out[facet] = buckets
		return rows.Err()
	}
	scalar := map[string]string{
		domain.FacetKind: "s.kind", domain.FacetModuleClass: "s.module_class", domain.FacetKey: "s.music_key", domain.FacetLicense: "s.license",
	}
	for facet, col := range scalar {
		a := &args{}
		conds, _, _ := where(a, q, facet)
		conds = append(conds, col+" IS NOT NULL")
		sql := `SELECT ` + col + ` AS v, count(*) FROM package_search s` + whereSQL(conds) + ` GROUP BY 1 ORDER BY 2 DESC, 1 LIMIT 100`
		if err := count(facet, sql, a); err != nil {
			return nil, err
		}
	}
	{
		a := &args{}
		conds, _, _ := where(a, q, domain.FacetBPM)
		sql := `SELECT b.v, count(*) FROM package_search s JOIN ` + bpmBuckets +
			` ON COALESCE(s.bpm_max, s.bpm) >= b.lo AND COALESCE(s.bpm_min, s.bpm) <= b.hi` + whereSQL(conds) +
			` GROUP BY b.v, b.lo ORDER BY b.lo`
		if err := count(domain.FacetBPM, sql, a); err != nil {
			return nil, err
		}
	}
	for facet, dom := range taxonomyFacets {
		a := &args{}
		conds, _, _ := where(a, q, facet)
		sql := `SELECT t.slug::text, count(DISTINCT s.package_id) FROM package_search s
			JOIN package_taxonomy t ON t.release_id = s.release_id AND t.domain = ` + a.add(dom) +
			whereSQL(conds) + ` GROUP BY 1 ORDER BY 2 DESC, 1 LIMIT 100`
		if err := count(facet, sql, a); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (r *SearchRepo) Samples(ctx context.Context, q domain.SampleQuery) ([]domain.SampleHit, int, error) {
	a := &args{}
	var conds []string
	if t := strings.TrimSpace(q.Text); t != "" {
		like := a.add("%" + likeEscape(strings.ToLower(t)) + "%")
		conds = append(conds, `(lower(si.name) LIKE `+like+` OR lower(si.path) LIKE `+like+` OR lower(p.name) LIKE `+like+
			` OR si.instrument = `+a.add(strings.ToLower(t))+` OR `+a.add(strings.ToLower(t))+` = ANY(si.tags))`)
	}
	if q.PackageID != nil {
		conds = append(conds, `si.package_id = `+a.add(*q.PackageID))
	}
	if len(q.Instruments) > 0 {
		conds = append(conds, `si.instrument = ANY(`+a.add(q.Instruments)+`::text[])`)
	}
	if q.ContentType != "" {
		conds = append(conds, `si.content_type = `+a.add(q.ContentType))
	}
	if len(q.Keys) > 0 {
		conds = append(conds, `si.music_key = ANY(`+a.add(q.Keys)+`::text[])`)
	}
	if q.BPMMin != nil {
		conds = append(conds, `si.bpm >= `+a.add(*q.BPMMin))
	}
	if q.BPMMax != nil {
		conds = append(conds, `si.bpm <= `+a.add(*q.BPMMax))
	}
	if q.DurationMax != nil {
		conds = append(conds, `si.duration_sec <= `+a.add(*q.DurationMax))
	}
	sql := `SELECT si.package_id, pub.slug, p.slug, p.name, r.version, si.path, si.name, si.digest, si.size_bytes, si.content_type,
		COALESCE(si.instrument, ''), COALESCE(si.role, ''), si.bpm, COALESCE(si.music_key, ''), si.duration_sec::float8, si.moods, si.tags,
		r.published_at, count(*) OVER ()
	FROM sample_index si
	JOIN package_search s ON s.release_id = si.release_id
	JOIN packages p ON p.id = si.package_id
	JOIN publishers pub ON pub.id = p.publisher_id
	JOIN releases r ON r.id = si.release_id` + whereSQL(conds) +
		` ORDER BY s.published_at DESC, si.path LIMIT ` + a.add(q.Limit) + ` OFFSET ` + a.add(q.Offset)
	rows, err := r.pool.Query(ctx, sql, a.list...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []domain.SampleHit{}
	total := 0
	for rows.Next() {
		var h domain.SampleHit
		if err := rows.Scan(&h.PackageID, &h.PublisherSlug, &h.PackageSlug, &h.PackageName, &h.Version, &h.Path, &h.Name, &h.Digest,
			&h.SizeBytes, &h.ContentType, &h.Instrument, &h.Role, &h.BPM, &h.Key, &h.DurationSec, &h.Moods, &h.Tags, &h.PublishedAt, &total); err != nil {
			return nil, 0, err
		}
		h.PackageCanon = h.PublisherSlug + "." + h.PackageSlug
		h.StorageKey, _ = domain.BlobKey(h.Digest)
		out = append(out, h)
	}
	return out, total, rows.Err()
}

func (r *SearchRepo) Vocabulary(ctx context.Context) ([]domain.VocabularyTerm, error) {
	rows, err := r.pool.Query(ctx, `SELECT taxonomy_version, domain, slug::text, label, COALESCE(parent_slug, ''), aliases
		FROM taxonomy_terms ORDER BY domain, slug`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.VocabularyTerm{}
	for rows.Next() {
		var t domain.VocabularyTerm
		if err := rows.Scan(&t.Version, &t.Domain, &t.Slug, &t.Label, &t.Parent, &t.Aliases); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
