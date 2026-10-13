package agentinit

import (
	"fmt"
	"os"
	"path/filepath"

	mnemoassets "github.com/jmeiracorbal/mnemo"
)

// globalSkills are the embedded skills installed under ~/.agents/skills and
// linked into the skill directory of every agent that needs one.
var globalSkills = []struct {
	name  string
	files []string
}{
	{name: globalSkillName, files: []string{"SKILL.md", "agents/openai.yaml"}},
	{name: "mnemo-doctor", files: []string{"SKILL.md", "agents/openai.yaml"}},
}

type skillSymlink struct {
	link   string
	target string
}

// InstallGlobalSkill copies the embedded mnemo skills to the canonical global
// path and links agent-specific skill directories to them.
func InstallGlobalSkill(home string) ([]string, error) {
	return installGlobalSkill(home, SupportedAgents)
}

// InstallGlobalSkillForAgents copies the embedded mnemo skills to the canonical
// global path and links only the requested agents' skill directories.
func InstallGlobalSkillForAgents(home string, agents []string) ([]string, error) {
	return installGlobalSkill(home, agents)
}

func installGlobalSkill(home string, agents []string) ([]string, error) {
	var updated []string
	for _, skill := range globalSkills {
		destRoot := filepath.Join(home, ".agents", "skills", skill.name)
		for _, rel := range skill.files {
			asset := "skills/" + skill.name + "/" + rel
			data, err := mnemoassets.SetupAssets.ReadFile(asset)
			if err != nil {
				return nil, fmt.Errorf("read embedded skill asset %s: %w", asset, err)
			}
			path := filepath.Join(destRoot, filepath.FromSlash(rel))
			if _, err := WriteFileIfChanged(path, data, 0644); err != nil {
				return nil, fmt.Errorf("write skill file %s: %w", path, err)
			}
			updated = append(updated, path)
		}
		for _, link := range globalSkillSymlinks(home, destRoot, skill.name, agents) {
			if err := ensureDirSymlink(link.link, link.target); err != nil {
				return nil, err
			}
			updated = append(updated, link.link)
		}
	}
	return updated, nil
}

func globalSkillSymlinks(home, destRoot, name string, agents []string) []skillSymlink {
	var symlinks []skillSymlink
	for _, agent := range agents {
		spec, ok := Spec(agent)
		if !ok || spec.Skill.SkillsDir == nil {
			continue
		}
		symlinks = append(symlinks, skillSymlink{
			link:   filepath.Join(spec.Skill.SkillsDir(home), name),
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
			if sameSymlinkTarget(link, target) {
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

// sameSymlinkTarget reports whether link already points at target, comparing
// resolved paths so symlinked parents (such as /var on macOS) still match.
func sameSymlinkTarget(link, target string) bool {
	if current, err := os.Readlink(link); err == nil && current == target {
		return true
	}
	current, err := filepath.EvalSymlinks(link)
	if err != nil {
		return false
	}
	resolved, err := filepath.EvalSymlinks(target)
	return err == nil && current == resolved
}
