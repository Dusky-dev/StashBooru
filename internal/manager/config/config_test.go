package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfig_GetMenuItemsIncludesMediaByDefaultAndPreservesCustomMenus(t *testing.T) {
	i := InitializeEmpty()
	assert.Equal(t, "media", i.GetMenuItems()[0])

	custom := []string{"images", "performers", "tags"}
	i.SetInterface(MenuItems, custom)
	assert.Equal(t, custom, i.GetMenuItems())

	i.SetInterface(MenuItems, []string{})
	assert.Empty(t, i.GetMenuItems())
}

func TestConfig_GetAllPluginConfiguration(t *testing.T) {
	i := InitializeEmpty()

	assert.Equal(t, i.GetAllPluginConfiguration(), map[string]map[string]interface{}{})

	i.SetPluginConfiguration("plugin1", map[string]interface{}{"key1": "value1"})

	assert.Equal(t, map[string]map[string]interface{}{
		"plugin1": {"key1": "value1"},
	}, i.GetAllPluginConfiguration())

	i.SetPluginConfiguration("plugin2", map[string]interface{}{"key2": "value2"})

	assert.Equal(t, map[string]map[string]interface{}{
		"plugin1": {"key1": "value1"},
		"plugin2": {"key2": "value2"},
	}, i.GetAllPluginConfiguration())

	// ensure SetPluginConfiguration overwrites existing configuration
	i.SetPluginConfiguration("plugin2", map[string]interface{}{"key3": "value3"})

	assert.Equal(t, map[string]map[string]interface{}{
		"plugin1": {"key1": "value1"},
		"plugin2": {"key3": "value3"},
	}, i.GetAllPluginConfiguration())
}

func TestAssociationInheritanceSettingsDefaultToEnabledAndCanBeDisabled(t *testing.T) {
	i := InitializeEmpty()
	assert.Equal(t, AssociationInheritanceSettings{
		Characters: true,
		Artists:    true,
		Copyrights: true,
		Tags:       true,
	}, i.GetAssociationInheritanceSettings())

	i.SetBool(AssociationInheritanceArtists, false)
	settings := i.GetAssociationInheritanceSettings()
	assert.True(t, settings.Characters)
	assert.False(t, settings.Artists)
	assert.True(t, settings.Copyrights)
	assert.True(t, settings.Tags)
}

func TestCopyrightAutoTagDefaults(t *testing.T) {
	c := InitializeEmpty()
	c.SetInterface(DefaultAutoTagSettings, &AutoTagMetadataOptions{Copyrights: []string{"*"}, Tags: []string{}})
	assert.Equal(t, []string{"*"}, c.GetDefaultAutoTagSettings().Copyrights)
	c.SetInterface(DefaultAutoTagSettings, map[string]interface{}{"performers": []string{"*"}})
	assert.Empty(t, c.GetDefaultAutoTagSettings().Copyrights, "old defaults do not silently enable Copyrights")
	assert.Equal(t, []string{"*"}, c.GetDefaultAutoTagSettings().Performers)
}
