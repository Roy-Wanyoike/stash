package identify

import (
	"errors"
	"reflect"
	"testing"

	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/models/mocks"
	"github.com/stretchr/testify/mock"
)

func Test_createMissingStudio(t *testing.T) {
	emptyEndpoint := ""
	validEndpoint := "validEndpoint"
	invalidEndpoint := "invalidEndpoint"
	remoteSiteID := "remoteSiteID"
	validName := "validName"
	invalidName := "invalidName"
	existingName := "existingName"
	createdID := 1
	existingStudioID := 42

	db := mocks.NewDatabase()

	db.Studio.On("Create", testCtx, mock.MatchedBy(func(p *models.CreateStudioInput) bool {
		return p.Name == validName
	})).Run(func(args mock.Arguments) {
		s := args.Get(1).(*models.CreateStudioInput)
		s.ID = createdID
	}).Return(nil)
	db.Studio.On("Create", testCtx, mock.MatchedBy(func(p *models.CreateStudioInput) bool {
		return p.Name == invalidName
	})).Return(errors.New("error creating studio"))

	// #7212: createMissingStudio now looks the studio up by name first;
	// default to no existing match so the create branch runs.
	db.Studio.On("FindByName", testCtx, mock.MatchedBy(func(name string) bool {
		return name == existingName
	}), mock.Anything).Return(&models.Studio{ID: existingStudioID, Name: existingName}, nil)
	db.Studio.On("FindByName", testCtx, mock.MatchedBy(func(name string) bool {
		return name != existingName
	}), mock.Anything).Return(nil, nil)

	db.Studio.On("UpdatePartial", testCtx, models.StudioPartial{
		ID: createdID,
		StashIDs: &models.UpdateStashIDs{
			StashIDs: []models.StashID{
				{
					Endpoint: invalidEndpoint,
					StashID:  remoteSiteID,
				},
			},
			Mode: models.RelationshipUpdateModeSet,
		},
	}).Return(nil, errors.New("error updating stash ids"))
	db.Studio.On("UpdatePartial", testCtx, models.StudioPartial{
		ID: createdID,
		StashIDs: &models.UpdateStashIDs{
			StashIDs: []models.StashID{
				{
					Endpoint: validEndpoint,
					StashID:  remoteSiteID,
				},
			},
			Mode: models.RelationshipUpdateModeSet,
		},
	}).Return(models.Studio{
		ID: createdID,
	}, nil)

	type args struct {
		endpoint string
		studio   *models.ScrapedStudio
	}
	tests := []struct {
		name    string
		args    args
		want    *int
		wantErr bool
	}{
		{
			"simple",
			args{
				emptyEndpoint,
				&models.ScrapedStudio{
					Name:         validName,
					RemoteSiteID: &remoteSiteID,
				},
			},
			&createdID,
			false,
		},
		{
			"error creating",
			args{
				emptyEndpoint,
				&models.ScrapedStudio{
					Name:         invalidName,
					RemoteSiteID: &remoteSiteID,
				},
			},
			nil,
			true,
		},
		{
			"valid stash id",
			args{
				validEndpoint,
				&models.ScrapedStudio{
					Name:         validName,
					RemoteSiteID: &remoteSiteID,
				},
			},
			&createdID,
			false,
		},
		{
			// #7212: studio with the same name already exists in the
			// database; reuse its ID instead of attempting a duplicate
			// INSERT that would violate the UNIQUE constraint on
			// studios(name).
			"existing studio reused",
			args{
				emptyEndpoint,
				&models.ScrapedStudio{
					Name: existingName,
				},
			},
			&existingStudioID,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := createMissingStudio(testCtx, tt.args.endpoint, db.Studio, tt.args.studio)
			if (err != nil) != tt.wantErr {
				t.Errorf("createMissingStudio() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("createMissingStudio() = %d, want %d", got, tt.want)
			}
		})
	}
}
