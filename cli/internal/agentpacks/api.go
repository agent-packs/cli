package agentpacks

import (
	"io"

	"github.com/sandeshh/agent-packs/cli/internal/author"
	"github.com/sandeshh/agent-packs/cli/internal/config"
	"github.com/sandeshh/agent-packs/cli/internal/install"
	"github.com/sandeshh/agent-packs/cli/internal/model"
	"github.com/sandeshh/agent-packs/cli/internal/plan"
	"github.com/sandeshh/agent-packs/cli/internal/policy"
	reg "github.com/sandeshh/agent-packs/cli/internal/registry"
	"github.com/sandeshh/agent-packs/cli/internal/resolve"
	"github.com/sandeshh/agent-packs/cli/internal/targets"
	"github.com/sandeshh/agent-packs/cli/internal/validate"
	"github.com/sandeshh/agent-packs/cli/internal/version"
)

type (
	Pack                = model.Pack
	Capability          = model.Capability
	CapabilityRef       = model.CapabilityRef
	CapabilityRefs      = model.CapabilityRefs
	Integrity           = model.Integrity
	SkillManifest       = model.SkillManifest
	PluginManifest      = model.PluginManifest
	InstallOptions      = model.InstallOptions
	Plan                = model.Plan
	PlanItem            = model.PlanItem
	Receipt             = model.Receipt
	Lockfile            = model.Lockfile
	LockEntry           = model.LockEntry
	SourceResolution    = model.SourceResolution
	TrustPolicy         = model.TrustPolicy
	RegistryIndex       = model.RegistryIndex
	IndexEntry          = model.IndexEntry
	RegistryConfig      = model.RegistryConfig
	TargetSpec          = model.TargetSpec
	AuditReport         = model.AuditReport
	ProjectConfig       = config.ProjectConfig
	InitOptions         = config.InitOptions
	OutdatedReport      = model.OutdatedReport
	InstalledSummary    = model.InstalledSummary
	CompatibilityResult = model.CompatibilityResult
	DependencyTree      = model.DependencyTree
	DependencyNode      = model.DependencyNode
	PublishReport       = model.PublishReport
	CheckEntry          = model.CheckEntry
	DoctorCheck         = model.DoctorCheck
	NewOptions          = author.NewOptions
)

var (
	TargetMatrix     = targets.TargetMatrix
	SkillTargets     = targets.SkillTargets
	ErrNotFound      = model.ErrNotFound
	ErrInstallFailed = model.ErrInstallFailed
)

func NormalizeAgent(agent string) string { return targets.NormalizeAgent(agent) }
func ValidAgent(agent string) bool       { return targets.ValidAgent(agent) }

func LoadPacks(registry string) ([]Pack, error)  { return reg.LoadPacks(registry) }
func LoadPack(path string) (Pack, error)         { return reg.LoadPack(path) }
func FindPack(registry, id string) (Pack, error) { return reg.FindPack(registry, id) }
func ResolvePack(defaultRegistry, home, ref string) (Pack, string, error) {
	return reg.ResolvePack(defaultRegistry, home, ref)
}
func ExpandPack(registry string, pack Pack, seen map[string]bool) (Pack, error) {
	return reg.ExpandPack(registry, pack, seen)
}
func ResolveCapabilityRef(registry, capabilityType string, ref CapabilityRef) (Capability, error) {
	return reg.ResolveCapabilityRef(registry, capabilityType, ref)
}
func FindCapability(registry, kind, id string) (Capability, error) {
	return reg.FindCapability(registry, kind, id)
}
func SkillCapability(id, path string, manifest SkillManifest) Capability {
	return reg.SkillCapability(id, path, manifest)
}
func PluginCapability(id, root string, manifest PluginManifest) Capability {
	return reg.PluginCapability(id, root, manifest)
}
func LoadSkillManifest(path string) (SkillManifest, error)   { return reg.LoadSkillManifest(path) }
func LoadPluginManifest(path string) (PluginManifest, error) { return reg.LoadPluginManifest(path) }
type SearchFilter = reg.SearchFilter

func MatchPacks(registry, query string) ([]Pack, error) { return reg.MatchPacks(registry, query) }
func FilteredMatchPacks(registry, query string, f SearchFilter) ([]Pack, error) {
	return reg.FilteredMatchPacks(registry, query, f)
}
func Search(registry, query string, out io.Writer) error     { return reg.Search(registry, query, out) }
func Show(registry, id string, out io.Writer) error          { return reg.Show(registry, id, out) }
func GenerateIndex(registry, outputPath string, out io.Writer) error {
	return reg.GenerateIndex(registry, outputPath, out)
}
func DependencyTreeForPack(registry, packRef string) (DependencyTree, error) {
	return reg.DependencyTree(registry, packRef)
}
func RegistryAdd(home, name, source string) error            { return reg.RegistryAdd(home, name, source) }
func RegistryRemove(home, name string) error                 { return reg.RegistryRemove(home, name) }
func RegistryList(home string, out io.Writer) error          { return reg.RegistryList(home, out) }
func LoadRegistryConfig(home string) (RegistryConfig, error) { return reg.LoadRegistryConfig(home) }
func ResolveRegistry(home, name string) (string, error)      { return reg.ResolveRegistry(home, name) }

func BuildInstallPlan(pack Pack, target, agent, only string) Plan {
	return plan.BuildInstallPlan(pack, target, agent, only)
}
func BuildInstallPlanWithOptions(pack Pack, target, agent, only string, options InstallOptions) Plan {
	return plan.BuildInstallPlanWithOptions(pack, target, agent, only, options)
}
func PrintPlan(p Plan, out io.Writer) { plan.PrintPlan(p, out) }

func Install(registry, home, packRef, target, agent, only string, executePlugins, dryRun bool, out io.Writer) error {
	return install.Install(registry, home, packRef, target, agent, only, executePlugins, dryRun, out)
}
func InstallWithOptions(registry, home, packRef, target, agent, only string, executePlugins, dryRun bool, options InstallOptions, out io.Writer) error {
	return install.InstallWithOptions(registry, home, packRef, target, agent, only, executePlugins, dryRun, options, out)
}
func InstallStandalone(registry, ref, kind, target, agent string, executePlugins, dryRun bool, options InstallOptions, out io.Writer) error {
	return install.InstallStandalone(registry, ref, kind, target, agent, executePlugins, dryRun, options, out)
}
func InstallStandaloneWithOverrides(registry, ref, kind, target, agent string, executePlugins, dryRun bool, options InstallOptions, installOverrides map[string]string, out io.Writer) error {
	return install.InstallStandaloneWithOverrides(registry, ref, kind, target, agent, executePlugins, dryRun, options, installOverrides, out)
}
func Upgrade(registry, home, packRef, target string, executePlugins bool, out io.Writer) error {
	return install.Upgrade(registry, home, packRef, target, executePlugins, out)
}
func UpgradeStandalone(target, id, kind string, executePlugins bool, out io.Writer) error {
	return install.UpgradeStandalone(target, id, kind, executePlugins, out)
}
func Rollback(target, packID string, out io.Writer) error {
	return install.Rollback(target, packID, out)
}
func ExecutePlan(p Plan, executePlugins bool) Plan { return install.ExecutePlan(p, executePlugins) }
func WriteReceipt(target string, pack Pack, p Plan) (string, error) {
	return install.WriteReceipt(target, pack, p)
}
func WriteStandaloneReceipt(target, kind, id string, pack Pack, p Plan) (string, error) {
	return install.WriteStandaloneReceipt(target, kind, id, pack, p)
}
func LoadReceipt(path string) (Receipt, error)         { return install.LoadReceipt(path) }
func WriteLockfile(packDir string, pack Pack) error    { return install.WriteLockfile(packDir, pack) }
func LoadLockfile(path string) (Lockfile, error)       { return install.LoadLockfile(path) }
func ListInstalled(target string, out io.Writer) error { return install.ListInstalled(target, out) }
func ListInstalledReceipts(target string) ([]InstalledSummary, error) {
	return install.ListInstalledReceipts(target)
}
func ListStandalone(target, kind string, out io.Writer) error {
	return install.ListStandalone(target, kind, out)
}
func Uninstall(target, packID string, out io.Writer) error {
	return install.Uninstall(target, packID, out)
}
func UninstallWithOptions(target, packID string, executePlugins bool, out io.Writer) error {
	return install.UninstallWithOptions(target, packID, executePlugins, out)
}
func UninstallStandalone(target, id, kind string, executePlugins bool, out io.Writer) error {
	return install.UninstallStandalone(target, id, kind, executePlugins, out)
}
func Outdated(registry, target string, out io.Writer) error {
	return install.Outdated(registry, target, out)
}
func GetOutdatedReport(registry, target string) (OutdatedReport, error) {
	return install.OutdatedReport(registry, target)
}
func PackDiff(registry, target, packRef string, out io.Writer) error {
	return install.PackDiff(registry, target, packRef, out)
}
func DriftCheck(target string, out io.Writer) error     { return install.DriftCheck(target, out) }
func DriftCheckJSON(target string, out io.Writer) error { return install.DriftCheckJSON(target, out) }
func CacheInfo(home string, out io.Writer) error    { return install.CacheInfo(home, out) }
func CachePrune(home string, clean bool, out io.Writer) error {
	return install.CachePrune(home, clean, out)
}
func Update(home string, all bool, out io.Writer) error { return install.Update(home, all, out) }

func PolicyCheck(registry, packRef, policyPath string, out io.Writer) error {
	return policy.PolicyCheck(registry, packRef, policyPath, out)
}
func Audit(registry, packRef string, out io.Writer) error {
	return policy.Audit(registry, packRef, out)
}
func AuditJSON(registry, packRef string, out io.Writer) error {
	return policy.AuditJSON(registry, packRef, out)
}
func BuildAuditReport(registry, packRef string) (AuditReport, error) {
	return policy.BuildAuditReport(registry, packRef)
}
func LoadTrustPolicy(path string) (TrustPolicy, error) { return policy.LoadTrustPolicy(path) }

func ValidatePath(path string, out io.Writer) error { return validate.ValidatePath(path, out) }
func ValidatePack(pack Pack) []string               { return validate.ValidatePack(pack) }
func ValidateCapability(capability Capability, prefix string) []string {
	return validate.ValidateCapability(capability, prefix)
}
func ValidateCapabilityRef(ref CapabilityRef, capabilityType, prefix string) []string {
	return validate.ValidateCapabilityRef(ref, capabilityType, prefix)
}
func ValidateCapabilityManifestPath(path string) []string {
	return validate.ValidateCapabilityManifestPath(path)
}
func ValidateSkillManifest(directoryName string, manifest SkillManifest) []string {
	return validate.ValidateSkillManifest(directoryName, manifest)
}
func ValidatePluginManifest(manifest PluginManifest) []string {
	return validate.ValidatePluginManifest(manifest)
}
func Lint(registry, packRef string, out io.Writer) error {
	return validate.Lint(registry, packRef, out)
}
func LintAll(registry string, out io.Writer) error { return validate.LintAll(registry, out) }
func Verify(registry, packRef string, out io.Writer) error {
	return validate.Verify(registry, packRef, out)
}
func VerifyAll(registry string, out io.Writer) error { return validate.VerifyAll(registry, out) }
func ResolveSources(registry, packRef string, out io.Writer) error {
	return validate.ResolveSources(registry, packRef, out)
}
func Licenses(registry, packRef string, out io.Writer) error {
	return validate.Licenses(registry, packRef, out)
}
func Attribution(registry, packRef string, out io.Writer) error {
	return validate.Attribution(registry, packRef, out)
}
func Compatibility(registry, packRef, agent string, out io.Writer) error {
	return validate.Compatibility(registry, packRef, agent, out)
}
func CompatibilityReport(registry, packRef, agent string) (CompatibilityResult, error) {
	return validate.CompatibilityReport(registry, packRef, agent)
}
func PublishCheck(registry, policyPath string, out io.Writer) error {
	return validate.PublishCheck(registry, policyPath, out)
}
func PublishReportForRegistry(registry, policyPath string) (PublishReport, error) {
	return validate.PublishReport(registry, policyPath)
}

func ResolveSource(source string) SourceResolution { return resolve.ResolveSource(source) }
func PrintTargetMatrix(out io.Writer) error { return targets.PrintTargetMatrix(out) }
func RegisterCustomTargets(home string)     { targets.RegisterCustomTargets(home) }
func AddCustomTarget(home, id, name, globalSkills, projectSkills string) error {
	return targets.AddCustomTarget(home, id, name, globalSkills, projectSkills)
}
func RemoveCustomTarget(home, id string) error              { return targets.RemoveCustomTarget(home, id) }
func ListCustomTargets(home string, out io.Writer) error    { return targets.ListCustomTargets(home, out) }
func VersionString() string                                 { return version.String() }
func InitProject(projectDir string, opts config.InitOptions) (string, error) {
	return config.Init(projectDir, opts)
}
func LoadProjectConfig(projectDir string) (ProjectConfig, error) {
	return config.LoadProjectConfig(projectDir)
}
func New(opts NewOptions) (string, error) { return author.New(opts) }
