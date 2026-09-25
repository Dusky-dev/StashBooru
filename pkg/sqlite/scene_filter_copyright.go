package sqlite

import "github.com/stashapp/stash/pkg/models"

func (qb *sceneFilterHandler) copyrightsCriterionHandler(copyrights *models.HierarchicalMultiCriterionInput) criterionHandlerFunc {
	h := joinedHierarchicalMultiCriterionHandlerBuilder{
		primaryTable:    sceneTable,
		foreignTable:    copyrightTable,
		foreignFK:       "copyright_id",
		relationsTable: copyrightRelationsTable,
		joinAs:          "copyrights_join",
		joinTable:       scenesCopyrightsTable,
		primaryFK:       sceneIDColumn,
	}

	return h.handler(copyrights)
}
