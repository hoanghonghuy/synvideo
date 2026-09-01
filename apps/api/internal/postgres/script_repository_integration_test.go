package postgres

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/creativeproposal"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/script"
)

func validScriptContent(prefix string) script.Content {
	duration := 120
	return script.Content{
		Sections: []script.Section{
			{Key: "intro", Heading: prefix + " Introduction", Body: prefix + " intro body"},
			{Key: "section-1", Heading: prefix + " Main Part", Body: prefix + " main body"},
			{Key: "outro", Heading: prefix + " Conclusion", Body: prefix + " outro body"},
		},
		EstimatedDurationSeconds: &duration,
		Notes:                    prefix + " notes",
	}
}

func TestScriptRepositoryIntegration(t *testing.T) {
	pool := integrationPool(t)
	projectRepo := NewProjectRepository(pool)
	proposalRepo := NewCreativeProposalRepository(pool)
	repo := NewScriptRepository(pool)

	ownerA := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	ownerB := uuid.MustParse("22222222-2222-4222-8222-222222222222")

	projectA, err := projectRepo.Create(context.Background(), ownerA, validIntegrationCreateInput("Script Project A"))
	if err != nil {
		t.Fatalf("create project A: %v", err)
	}

	// Create Proposal v1 on project A as draft
	propV1, err := proposalRepo.CreateDraft(context.Background(), ownerA, projectA.ID, creativeproposal.CreateDraftInput{
		SourceBriefRevision: 1,
		Content:             validProposalContent("Proposal V1"),
	})
	if err != nil {
		t.Fatalf("create proposal draft: %v", err)
	}

	// 2. Non-approved Proposal -> rejected without Script creation
	t.Run("2. Non-approved Proposal rejected", func(t *testing.T) {
		_, err := repo.CreateDraft(context.Background(), ownerA, projectA.ID, script.CreateDraftInput{
			SourceProposalVersion: propV1.Version,
			Content:               validScriptContent("Draft with unapproved proposal"),
		})
		if !errors.Is(err, script.ErrProposalNotApproved) {
			t.Fatalf("expected ErrProposalNotApproved when proposal is in draft status, got %v", err)
		}
	})

	// Now approve Proposal v1
	propV1Approved, err := proposalRepo.Approve(context.Background(), ownerA, projectA.ID, propV1.Version, propV1.Revision)
	if err != nil {
		t.Fatalf("approve proposal: %v", err)
	}
	if propV1Approved.Status != creativeproposal.StatusApproved {
		t.Fatalf("proposal not approved: %s", propV1Approved.Status)
	}

	// 1. Approved Proposal -> first Script draft v1/revision 1/draft
	var scriptV1 script.Script
	t.Run("1. First Script draft from approved Proposal", func(t *testing.T) {
		created, err := repo.CreateDraft(context.Background(), ownerA, projectA.ID, script.CreateDraftInput{
			SourceProposalVersion: propV1Approved.Version,
			Content:               validScriptContent("Script V1"),
		})
		if err != nil {
			t.Fatalf("create first script draft: %v", err)
		}
		if created.Version != 1 || created.Revision != 1 || created.Status != script.StatusDraft {
			t.Fatalf("unexpected first script draft: %#v", created)
		}
		if created.SourceProposalVersion != propV1Approved.Version {
			t.Fatalf("unexpected source proposal version: %d", created.SourceProposalVersion)
		}
		if created.ContentLocale != string(projectA.Locale) {
			t.Fatalf("expected content locale %s, got %s", projectA.Locale, created.ContentLocale)
		}
		scriptV1 = created
	})

	// 5. Current revision update increments exactly once
	t.Run("5. Revision update increments once", func(t *testing.T) {
		newContent := validScriptContent("Script V1 Edited")
		updated, err := repo.UpdateDraft(context.Background(), ownerA, projectA.ID, scriptV1.Version, script.PutInput{
			Revision: &scriptV1.Revision,
			Content:  newContent,
		})
		if err != nil {
			t.Fatalf("update draft: %v", err)
		}
		if updated.Revision != scriptV1.Revision+1 {
			t.Fatalf("expected revision %d, got %d", scriptV1.Revision+1, updated.Revision)
		}
		if updated.Sections[0].Heading != "Script V1 Edited Introduction" {
			t.Fatalf("heading not updated: %s", updated.Sections[0].Heading)
		}
		scriptV1 = updated
	})

	// 6. Stale update / approval -> conflict
	t.Run("6. Stale update and approval conflict", func(t *testing.T) {
		staleRevision := scriptV1.Revision - 1
		_, err := repo.UpdateDraft(context.Background(), ownerA, projectA.ID, scriptV1.Version, script.PutInput{
			Revision: &staleRevision,
			Content:  validScriptContent("Stale Edit"),
		})
		if !errors.Is(err, script.ErrStaleRevision) {
			t.Fatalf("expected ErrStaleRevision on update with stale revision, got %v", err)
		}

		_, err = repo.Approve(context.Background(), ownerA, projectA.ID, scriptV1.Version, staleRevision)
		if !errors.Is(err, script.ErrStaleRevision) {
			t.Fatalf("expected ErrStaleRevision on approve with stale revision, got %v", err)
		}
	})

	// 3. Second draft -> monotonic version + prior active draft superseded
	var scriptV2 script.Script
	t.Run("3. Second draft supersedes prior unapproved draft", func(t *testing.T) {
		v2, err := repo.CreateDraft(context.Background(), ownerA, projectA.ID, script.CreateDraftInput{
			SourceProposalVersion: propV1Approved.Version,
			Content:               validScriptContent("Script V2"),
		})
		if err != nil {
			t.Fatalf("create second script draft: %v", err)
		}
		if v2.Version != 2 || v2.Revision != 1 || v2.Status != script.StatusDraft {
			t.Fatalf("unexpected script v2: %#v", v2)
		}

		// Check prior v1 is now superseded
		v1Fetched, err := repo.GetByVersion(context.Background(), ownerA, projectA.ID, 1)
		if err != nil {
			t.Fatalf("get v1: %v", err)
		}
		if v1Fetched.Status != script.StatusSuperseded {
			t.Fatalf("expected v1 to be superseded, got %s", v1Fetched.Status)
		}

		// Updating superseded draft should fail with ErrScriptImmutable
		rev1 := v1Fetched.Revision
		_, err = repo.UpdateDraft(context.Background(), ownerA, projectA.ID, 1, script.PutInput{
			Revision: &rev1,
			Content:  validScriptContent("Edit superseded"),
		})
		if !errors.Is(err, script.ErrScriptImmutable) {
			t.Fatalf("expected ErrScriptImmutable on superseded draft edit, got %v", err)
		}
		scriptV2 = v2
	})

	// 4. Approved Script remains immutable and preserved
	t.Run("4. Approved Script remains immutable", func(t *testing.T) {
		approvedV2, err := repo.Approve(context.Background(), ownerA, projectA.ID, scriptV2.Version, scriptV2.Revision)
		if err != nil {
			t.Fatalf("approve v2: %v", err)
		}
		if approvedV2.Status != script.StatusApproved || approvedV2.ApprovedAt == nil {
			t.Fatalf("expected approved status and approved_at set: %#v", approvedV2)
		}

		// Mutating approved draft fails
		rev := approvedV2.Revision
		_, err = repo.UpdateDraft(context.Background(), ownerA, projectA.ID, approvedV2.Version, script.PutInput{
			Revision: &rev,
			Content:  validScriptContent("Mutate approved"),
		})
		if !errors.Is(err, script.ErrScriptImmutable) {
			t.Fatalf("expected ErrScriptImmutable modifying approved script, got %v", err)
		}

		// Approving already approved draft fails
		_, err = repo.Approve(context.Background(), ownerA, projectA.ID, approvedV2.Version, rev)
		if !errors.Is(err, script.ErrScriptImmutable) {
			t.Fatalf("expected ErrScriptImmutable approving already approved script, got %v", err)
		}

		// Creating a v3 preserves approved v2 (does NOT supersede v2)
		v3, err := repo.CreateDraft(context.Background(), ownerA, projectA.ID, script.CreateDraftInput{
			SourceProposalVersion: propV1Approved.Version,
			Content:               validScriptContent("Script V3"),
		})
		if err != nil {
			t.Fatalf("create v3: %v", err)
		}
		if v3.Version != 3 {
			t.Fatalf("expected v3, got %d", v3.Version)
		}

		v2Check, err := repo.GetByVersion(context.Background(), ownerA, projectA.ID, 2)
		if err != nil {
			t.Fatalf("get v2: %v", err)
		}
		if v2Check.Status != script.StatusApproved {
			t.Fatalf("expected v2 to remain approved, got %s", v2Check.Status)
		}
	})

	// 8. Owner isolation and non-disclosure
	t.Run("8. Owner isolation", func(t *testing.T) {
		// Owner B cannot get, list, update, approve, create on Project A
		_, err := repo.GetByVersion(context.Background(), ownerB, projectA.ID, 1)
		if !errors.Is(err, script.ErrNotFound) {
			t.Fatalf("expected ErrNotFound for Owner B get, got %v", err)
		}

		listB, err := repo.ListVersions(context.Background(), ownerB, projectA.ID)
		if !errors.Is(err, script.ErrNotFound) {
			t.Fatalf("expected ErrNotFound for Owner B list, got %v (len=%d)", err, len(listB))
		}

		rev := 1
		_, err = repo.UpdateDraft(context.Background(), ownerB, projectA.ID, 3, script.PutInput{
			Revision: &rev,
			Content:  validScriptContent("Owner B edit"),
		})
		if !errors.Is(err, script.ErrNotFound) {
			t.Fatalf("expected ErrNotFound for Owner B update, got %v", err)
		}

		_, err = repo.Approve(context.Background(), ownerB, projectA.ID, 3, rev)
		if !errors.Is(err, script.ErrNotFound) {
			t.Fatalf("expected ErrNotFound for Owner B approve, got %v", err)
		}

		_, err = repo.CreateDraft(context.Background(), ownerB, projectA.ID, script.CreateDraftInput{
			SourceProposalVersion: propV1Approved.Version,
			Content:               validScriptContent("Owner B draft"),
		})
		if !errors.Is(err, script.ErrNotFound) {
			t.Fatalf("expected ErrNotFound for Owner B create, got %v", err)
		}
	})

	// 9. Version list newest first
	t.Run("9. Version list newest first", func(t *testing.T) {
		list, err := repo.ListVersions(context.Background(), ownerA, projectA.ID)
		if err != nil {
			t.Fatalf("list versions: %v", err)
		}
		if len(list) != 3 {
			t.Fatalf("expected 3 versions, got %d", len(list))
		}
		if list[0].Version != 3 || list[1].Version != 2 || list[2].Version != 1 {
			t.Fatalf("expected versions [3, 2, 1], got [%d, %d, %d]", list[0].Version, list[1].Version, list[2].Version)
		}
	})

	// 7. Concurrent CreateDraft -> unique versions + one active draft
	t.Run("7. Concurrent CreateDraft unique versions and one active draft", func(t *testing.T) {
		projectConc, err := projectRepo.Create(context.Background(), ownerA, validIntegrationCreateInput("Script Concurrency Project"))
		if err != nil {
			t.Fatalf("create concurrency project: %v", err)
		}
		propConc, err := proposalRepo.CreateDraft(context.Background(), ownerA, projectConc.ID, creativeproposal.CreateDraftInput{
			SourceBriefRevision: 1,
			Content:             validProposalContent("Concurrency Proposal"),
		})
		if err != nil {
			t.Fatalf("create proposal: %v", err)
		}
		propConcApproved, err := proposalRepo.Approve(context.Background(), ownerA, projectConc.ID, propConc.Version, propConc.Revision)
		if err != nil {
			t.Fatalf("approve proposal: %v", err)
		}

		concurrency := 6
		var wg sync.WaitGroup
		wg.Add(concurrency)
		errs := make([]error, concurrency)
		results := make([]script.Script, concurrency)

		for i := 0; i < concurrency; i++ {
			idx := i
			go func() {
				defer wg.Done()
				res, cErr := repo.CreateDraft(context.Background(), ownerA, projectConc.ID, script.CreateDraftInput{
					SourceProposalVersion: propConcApproved.Version,
					Content:               validScriptContent("Concurrent Script " + string(rune('A'+idx))),
				})
				errs[idx] = cErr
				results[idx] = res
			}()
		}
		wg.Wait()

		for i, cErr := range errs {
			if cErr != nil {
				t.Fatalf("goroutine %d failed: %v", i, cErr)
			}
		}

		// Verify version uniqueness
		versionMap := make(map[int]bool)
		for _, r := range results {
			if versionMap[r.Version] {
				t.Fatalf("duplicate version allocated: %d", r.Version)
			}
			versionMap[r.Version] = true
		}

		// Verify exactly 1 active draft exists
		list, err := repo.ListVersions(context.Background(), ownerA, projectConc.ID)
		if err != nil {
			t.Fatalf("list versions: %v", err)
		}
		draftCount := 0
		supersededCount := 0
		for _, s := range list {
			if s.Status == script.StatusDraft {
				draftCount++
			} else if s.Status == script.StatusSuperseded {
				supersededCount++
			}
		}
		if draftCount != 1 {
			t.Fatalf("expected exactly 1 active draft, got %d", draftCount)
		}
		if supersededCount != concurrency-1 {
			t.Fatalf("expected %d superseded drafts, got %d", concurrency-1, supersededCount)
		}
	})

	t.Run("10. Multibyte Unicode character persistence roundtrip", func(t *testing.T) {
		ownerU := uuid.New()
		projectU, err := projectRepo.Create(context.Background(), ownerU, project.CreateInput{
			Title:         "Dự án kịch bản tiếng Việt",
			ContentFormat: project.ContentFormatShort,
			AspectRatio:   project.AspectRatio9x16,
			Locale:        project.LocaleVI,
		})
		if err != nil {
			t.Fatalf("create project: %v", err)
		}

		propU, err := proposalRepo.CreateDraft(context.Background(), ownerU, projectU.ID, creativeproposal.CreateDraftInput{
			SourceBriefRevision: 1,
			Content:             validProposalContent("Đề xuất video ngắn tiếng Việt"),
		})
		if err != nil {
			t.Fatalf("create proposal: %v", err)
		}
		propUApproved, err := proposalRepo.Approve(context.Background(), ownerU, projectU.ID, propU.Version, propU.Revision)
		if err != nil {
			t.Fatalf("approve proposal: %v", err)
		}

		// Create heading with 300 Vietnamese runes (e.g. 600+ UTF-8 bytes)
		unicodeHeadingRunes := make([]rune, 300)
		for i := range unicodeHeadingRunes {
			unicodeHeadingRunes[i] = 'ế'
		}
		unicodeHeading := string(unicodeHeadingRunes)

		// Create body with 5000 Vietnamese runes (e.g. 15000 UTF-8 bytes)
		unicodeBodyRunes := make([]rune, 5000)
		for i := range unicodeBodyRunes {
			unicodeBodyRunes[i] = 'ả'
		}
		unicodeBody := string(unicodeBodyRunes)

		// Create notes with 4000 Vietnamese runes (e.g. 12000 UTF-8 bytes)
		unicodeNotesRunes := make([]rune, 4000)
		for i := range unicodeNotesRunes {
			unicodeNotesRunes[i] = 'ộ'
		}
		unicodeNotes := string(unicodeNotesRunes)

		content := script.Content{
			Sections: []script.Section{
				{
					Key:     "phan-mo-dau",
					Heading: unicodeHeading,
					Body:    unicodeBody,
				},
			},
			Notes: unicodeNotes,
		}

		created, err := repo.CreateDraft(context.Background(), ownerU, projectU.ID, script.CreateDraftInput{
			SourceProposalVersion: propUApproved.Version,
			Content:               content,
		})
		if err != nil {
			t.Fatalf("create unicode script draft: %v", err)
		}

		fetched, err := repo.GetByVersion(context.Background(), ownerU, projectU.ID, created.Version)
		if err != nil {
			t.Fatalf("get unicode script: %v", err)
		}

		if len([]rune(fetched.Sections[0].Heading)) != 300 || fetched.Sections[0].Heading != unicodeHeading {
			t.Fatalf("heading unicode runes mismatch: len=%d", len([]rune(fetched.Sections[0].Heading)))
		}
		if len([]rune(fetched.Sections[0].Body)) != 5000 || fetched.Sections[0].Body != unicodeBody {
			t.Fatalf("body unicode runes mismatch: len=%d", len([]rune(fetched.Sections[0].Body)))
		}
		if len([]rune(fetched.Notes)) != 4000 || fetched.Notes != unicodeNotes {
			t.Fatalf("notes unicode runes mismatch: len=%d", len([]rune(fetched.Notes)))
		}
	})
}
