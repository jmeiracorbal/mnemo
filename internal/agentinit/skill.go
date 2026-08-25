package agentinit

import (
	"fmt"
	"os"
	"path/filepath"

	mnemoassets "github.com/jmeiracorbal/mnemo"
)

var globalSkillFiles = []struct {
	asset string
	rel   string
	mode  os.FileMode
}{
	{asset: "skills/mnemo-memory/SKILL.md", rel: "SKILL.md", mode: 0644},
	{asset: "skills/mnemo-memory/agents/openai.yaml", rel: "agents/openai.yaml", mode: 0644},
}

type skillSymlink struct {
	link   string
	target string
}

// InstallGlobalSkill copies the embedded mnemo-memory skill to the canonical
// global path and links agent-specific skill directories to it.
func InstallGlobalSkill(home string) ([]string, error) {
	destRoot := filepath.Join(home, ".agents", "skills", globalSkillName)
	var updated []string

	for _, file := range globalSkillFiles {
		data, err := mnemoassets.SetupAssets.ReadFile(file.asset)
		if err != nil {
			return nil, fmt.Errorf("read embedded skill asset %s: %w", file.asset, err)
		}
		path := filepath.Join(destRoot, file.rel)
		if err := WriteFile(path, data); err != nil {
			return nil, fmt.Errorf("write skill file %s: %w", path, err)
		}
		if err := os.Chmod(path, file.mode); err != nil {
			return nil, fmt.Errorf("chmod skill file %s: %w", path, err)
		}
		updated = append(updated, path)
	}

	for _, spec := range globalSkillSymlinks(home, destRoot) {
		if err := ensureDirSymlink(spec.link, spec.target); err != nil {
			return nil, err
		}
		updated = append(updated, spec.link)
	}

	return updated, nil
}

func globalSkillSymlinks(home, destRoot string) []skillSymlink {
	var symlinks []skillSymlink
	for _, spec := range agentSpecs {
		if spec.Skill.GlobalLinkPath == nil {
			continue
		}
		symlinks = append(symlinks, skillSymlink{
			link:   spec.Skill.GlobalLinkPath(home),
			target: destRoot,
		})
	}
	return symlinks
}

func ensureDirSymlink(link, target string) error {
	if err := os.MkdirAll(filepath.Dir(link), 0755); err != nil {
		return fmt.Errorf("create skill link parent %s: %w", link, err)
	}

	info, err := os.Lstat(link)
	switch {
	case err == nil:
		if info.Mode()&os.ModeSymlink != 0 {
			current, readErr := filepath.EvalSymlinks(link)
			if readErr == nil && current == target {
				return nil
			}
			if removeErr := os.Remove(link); removeErr != nil {
				return fmt.Errorf("replace skill symlink %s: %w", link, removeErr)
			}
			break
		}
		return fmt.Errorf("skill link path %s exists and is not a symlink", link)
	case !os.IsNotExist(err):
		return fmt.Errorf("stat skill link %s: %w", link, err)
	}

	if err := os.Symlink(target, link); err != nil {
		return fmt.Errorf("create skill symlink %s -> %s: %w", link, target, err)
	}
	return nil
}
