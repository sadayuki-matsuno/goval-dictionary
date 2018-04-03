package rdb

import (
	"fmt"

	"github.com/inconshreveable/log15"
	"github.com/jinzhu/gorm"
	"github.com/k0kubun/pp"
	"github.com/kotakanbe/goval-dictionary/config"
	"github.com/kotakanbe/goval-dictionary/models"
)

// Amazon is a struct for DBAccess
type Amazon struct {
	Family string
}

// NewAmazon creates DBAccess
func NewAmazon() *Amazon {
	return &Amazon{Family: config.Amazon}
}

// Name return family name
func (o *Amazon) Name() string {
	return o.Family
}

// InsertOval inserts Amazon ALAS information as OVAL format
func (o *Amazon) InsertOval(root *models.Root, meta models.FetchMeta, driver *gorm.DB) error {
	log15.Debug("in Amazon")
	tx := driver.Begin()

	old := models.Root{}
	r := tx.Where(&models.Root{Family: root.Family}).First(&old)
	if !r.RecordNotFound() {
		// Delete data related to root passed in arg
		defs := []models.Definition{}
		driver.Model(&old).Related(&defs, "Definitions")
		for _, def := range defs {
			adv := models.Advisory{}
			driver.Model(&def).Related(&adv, "Advisory")
			if err := tx.Unscoped().Where("advisory_id = ?", adv.ID).Delete(&models.Cve{}).Error; err != nil {
				tx.Rollback()
				return fmt.Errorf("Failed to delete: %s", err)
			}
			if err := tx.Unscoped().Where("definition_id = ?", def.ID).Delete(&models.Advisory{}).Error; err != nil {
				tx.Rollback()
				return fmt.Errorf("Failed to delete: %s", err)
			}
			if err := tx.Unscoped().Where("definition_id= ?", def.ID).Delete(&models.Package{}).Error; err != nil {
				tx.Rollback()
				return fmt.Errorf("Failed to delete: %s", err)
			}
			if err := tx.Unscoped().Where("definition_id = ?", def.ID).Delete(&models.Reference{}).Error; err != nil {
				tx.Rollback()
				return fmt.Errorf("Failed to delete: %s", err)
			}
		}
		if err := tx.Unscoped().Where("root_id = ?", old.ID).Delete(&models.Definition{}).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("Failed to delete: %s", err)
		}
		if err := tx.Unscoped().Where("id = ?", old.ID).Delete(&models.Root{}).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("Failed to delete: %s", err)
		}
	}

	if err := tx.Create(&root).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("Failed to insert. cve: %s, err: %s",
			pp.Sprintf("%v", root), err)
	}

	return tx.Commit().Error
}

// GetByPackName select definitions by packName
func (o *Amazon) GetByPackName(osVer, packName string, driver *gorm.DB) ([]models.Definition, error) {
	osVer = majorMinor(osVer)
	packs := []models.Package{}
	if err := driver.Where(&models.Package{Name: packName}).Find(&packs).Error; err != nil {
		return nil, err
	}

	defs := []models.Definition{}
	for _, p := range packs {
		def := models.Definition{}
		if err := driver.Where("id = ?", p.DefinitionID).Find(&def).Error; err != nil {
			return nil, err
		}

		root := models.Root{}
		if err := driver.Where("id = ?", def.RootID).Find(&root).Error; err != nil {
			return nil, err
		}

		// Amazon has no version information
		if root.Family == config.Amazon {
			defs = append(defs, def)
		}

		for i, def := range defs {
			adv := models.Advisory{}
			if err := driver.Model(&def).Related(&adv, "Advisory").Error; err != nil {
				return nil, err
			}

			cves := []models.Cve{}
			if err := driver.Model(&adv).Related(&cves, "Cves").Error; err != nil {
				return nil, err
			}

			adv.Cves = cves
			defs[i].Advisory = adv

			packs := []models.Package{}
			if err := driver.Model(&def).Related(&packs, "AffectedPacks").Error; err != nil {
				return nil, err
			}
			defs[i].AffectedPacks = packs

			refs := []models.Reference{}
			if err := driver.Model(&def).Related(&refs, "References").Error; err != nil {
				return nil, err
			}
			defs[i].References = refs
		}
	}

	return defs, nil
}

// GetByCveID select definitions by CveID
func (o *Amazon) GetByCveID(osVer, cveID string, driver *gorm.DB) ([]models.Definition, error) {
	osVer = majorMinor(osVer)
	cves := []models.Cve{}
	if err := driver.Where(&models.Cve{CveID: cveID}).Find(&cves).Error; err != nil {
		return nil, err
	}

	defs := []models.Definition{}
	for _, cve := range cves {
		adv := models.Advisory{}
		if err := driver.Where("id = ?", cve.AdvisoryID).Find(&adv).Error; err != nil {
			return nil, err
		}

		def := models.Definition{}
		if err := driver.Where("id = ?", adv.DefinitionID).Find(&def).Error; err != nil {
			return nil, err
		}

		root := models.Root{}
		if err := driver.Where("id = ?", def.RootID).Find(&root).Error; err != nil {
			return nil, err
		}

		// Amazon has no version information
		if root.Family == config.Amazon {
			defs = append(defs, def)
		}
	}

	for i, def := range defs {
		adv := models.Advisory{}
		if err := driver.Model(&def).Related(&adv, "Advisory").Error; err != nil {
			return nil, err
		}

		cves := []models.Cve{}
		if err := driver.Model(&adv).Related(&cves, "Cves").Error; err != nil {
			return nil, err
		}
		adv.Cves = cves

		defs[i].Advisory = adv

		packs := []models.Package{}
		if err := driver.Model(&def).Related(&packs, "AffectedPacks").Error; err != nil {
			return nil, err
		}
		defs[i].AffectedPacks = packs

		refs := []models.Reference{}
		if err := driver.Model(&def).Related(&refs, "References").Error; err != nil {
			return nil, err
		}
		defs[i].References = refs
	}

	return defs, nil
}
