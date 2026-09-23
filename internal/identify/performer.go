package identify

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
)

type PerformerCreator interface {
	models.PerformerCreator
	models.PerformerFinder
	UpdateImage(ctx context.Context, performerID int, image []byte) error
}

func getPerformerID(ctx context.Context, endpoint string, w PerformerCreator, p *models.ScrapedPerformer, createMissing bool, skipSingleNamePerformers bool) (*int, error) {
	if p.StoredID != nil {
		// existing performer, just add it
		performerID, err := strconv.Atoi(*p.StoredID)
		if err != nil {
			return nil, fmt.Errorf("error converting performer ID %s: %w", *p.StoredID, err)
		}

		return &performerID, nil
	} else if createMissing && p.Name != nil { // name is mandatory
		// skip single name performers with no disambiguation
		if skipSingleNamePerformers && !strings.Contains(*p.Name, " ") && (p.Disambiguation == nil || len(*p.Disambiguation) == 0) {
			return nil, ErrSkipSingleNamePerformer
		}
		return createMissingPerformer(ctx, endpoint, w, p)
	}

	return nil, nil
}

func createMissingPerformer(ctx context.Context, endpoint string, w PerformerCreator, p *models.ScrapedPerformer) (*int, error) {
	// #7212: The match logic in pkg/match only runs for stash-box sources,
	// and even then it skips ambiguous name matches (more than one
	// existing performer with the same name). For scraper sources (e.g.
	// Redgifs) the scraped performer arrives here with StoredID=nil even
	// when a performer with the same name already exists. Creating a new
	// row in that case violates the UNIQUE constraint on
	// performers(name) / performers(name, disambiguation). Look the
	// performer up by name (and disambiguation) first; only create a new
	// row if no match is found.
	if p.Name != nil {
		existing, err := w.FindByNames(ctx, []string{*p.Name}, true)
		if err != nil {
			return nil, fmt.Errorf("error looking up existing performer by name: %w", err)
		}
		if id := matchExistingPerformer(existing, p); id != nil {
			logger.Debugf("Performer %q already exists with id %d, reusing", *p.Name, *id)
			return id, nil
		}
	}

	newPerformer := p.ToPerformer(endpoint, nil)
	performerImage, err := p.GetImage(ctx, nil)
	if err != nil {
		return nil, err
	}

	err = w.Create(ctx, &models.CreatePerformerInput{Performer: newPerformer})
	if err != nil {
		return nil, fmt.Errorf("error creating performer: %w", err)
	}

	// update image table
	if len(performerImage) > 0 {
		if err := w.UpdateImage(ctx, newPerformer.ID, performerImage); err != nil {
			return nil, err
		}
	}

	return &newPerformer.ID, nil
}

// matchExistingPerformer returns the ID of the first performer in the slice
// whose disambiguation matches the scraped value (both empty, or both
// non-empty and equal). The performers table has UNIQUE constraints on
// (name) where disambiguation IS NULL and on (name, disambiguation) where
// disambiguation IS NOT NULL, so this is the set of rows that would
// conflict with a fresh INSERT for the scraped performer.
func matchExistingPerformer(performers []*models.Performer, p *models.ScrapedPerformer) *int {
	var scrapedDisambig string
	if p.Disambiguation != nil {
		scrapedDisambig = *p.Disambiguation
	}

	for _, existing := range performers {
		if existing.Disambiguation == scrapedDisambig {
			id := existing.ID
			return &id
		}
	}

	return nil
}
