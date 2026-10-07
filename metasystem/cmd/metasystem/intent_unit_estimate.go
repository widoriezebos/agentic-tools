package main

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/designgate"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/project"
)

var errEstimateChanged = errors.New("the working design differs from the accepted page on origin/main")

func (inv *intentInvocation) unitEstimate(f designgate.Facts, unit string) (*launch.UnitEstimate, error) {
	if f.Error != nil {
		return nil, f.Error
	}
	var estimate *launch.UnitEstimate
	for _, design := range f.Designs {
		if design.Status != "accepted" {
			continue
		}
		path := filepath.FromSlash(design.Path)
		if !filepath.IsAbs(path) {
			path = filepath.Join(inv.layout.GitRoot, path)
		}
		relative, err := filepath.Rel(inv.layout.GitRoot, path)
		if err != nil {
			return nil, err
		}
		data, err := inv.work().git(inv.layout.GitRoot, "show", "origin/main:"+filepath.ToSlash(relative))
		if err != nil {
			return nil, fmt.Errorf("the accepted design %s cannot be read on origin/main: %w", design.Path, err)
		}
		record, problems, declared := project.ParseRecord(design.Path, string(data))
		if !declared || len(problems) != 0 || record.Kind != project.KindDesign || record.ID != design.ID || record.Status != "accepted" || !slices.Contains(record.Goals, f.Goal) {
			return nil, fmt.Errorf("the design %s is not accepted for this goal on origin/main", design.Path)
		}
		design.SHA256 = fmt.Sprintf("%x", sha256.Sum256(data))
		if local, err := os.ReadFile(path); err == nil && fmt.Sprintf("%x", sha256.Sum256(local)) != design.SHA256 {
			return nil, errEstimateChanged
		}
		body, err := inv.designBodyDigest(design)
		if err != nil {
			return nil, err
		}
		inEstimates := false
		var columns []string
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "#") {
				inEstimates = line == "## Estimates"
				columns = nil
			}
			if !inEstimates || !strings.HasPrefix(line, "|") {
				continue
			}
			cells := strings.Split(strings.Trim(line, "|"), "|")
			for index := range cells {
				cells[index] = strings.TrimSpace(cells[index])
			}
			if cells[0] == "Unit" {
				columns = cells
				continue
			}
			if cells[0] != unit || len(cells) != len(columns) {
				continue
			}
			row := &launch.UnitEstimate{DesignID: design.ID, SourceSHA256: design.SHA256, BodySHA256: body, Unit: unit}
			for index, column := range columns {
				if column != "Estimated elapsed minutes" && column != "Expected impacted-check minutes" {
					continue
				}
				if column == "Expected impacted-check minutes" && (cells[index] == "" || cells[index] == "-") {
					continue
				}
				value, err := strconv.ParseFloat(cells[index], 64)
				if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
					return nil, fmt.Errorf("the accepted estimate for %s has unreadable minutes", unit)
				}
				if column == "Estimated elapsed minutes" {
					row.ElapsedMinutes = value
				} else {
					row.CheckMinutes = &value
				}
			}
			if estimate != nil || row.ElapsedMinutes <= 0 {
				return nil, fmt.Errorf("the accepted estimate for %s is missing or ambiguous", unit)
			}
			estimate = row
		}
	}
	return estimate, nil
}

// freezeUnitEstimate serializes first admission across worktrees of one ledger.
func (inv *intentInvocation) freezeUnitEstimate(plan *launch.UnitPlan, f designgate.Facts, person bool) error {
	identity, err := inv.designGate().identity(inv.layout.InstallationRoot)
	if err != nil || !designGateIdentity.MatchString(identity) {
		return fmt.Errorf("the goal ledger identity is unavailable")
	}
	key := fmt.Sprintf("%x", sha256.Sum256([]byte(identity+"\x00"+plan.Goal+"\x00"+plan.Unit)))
	root := filepath.Dir(filepath.Dir(plan.Path))
	path := filepath.Join(root, ".estimates", key+".json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	held, err := lock.File(path+".lock", 0o600, lock.TryExclusive)
	if err != nil {
		return err
	}
	defer held.Release()
	file, err := os.Open(path)
	if err == nil {
		defer file.Close()
		decoder := json.NewDecoder(file)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&plan.Estimate); err != nil {
			return err
		}
		if decoder.Decode(new(any)) != io.EOF {
			return fmt.Errorf("the retained estimate has trailing data")
		}
		if plan.Estimate != nil {
			return plan.Estimate.Validate(plan.Unit)
		}
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	plan.Estimate, err = inv.unitEstimate(f, plan.Unit)
	if err != nil || plan.Estimate == nil {
		return err
	}
	data, writeErr := json.Marshal(plan.Estimate)
	if writeErr == nil {
		_, writeErr = atomicfile.WriteText(path, string(data)+"\n", root)
	}
	return errors.Join(err, writeErr)
}
