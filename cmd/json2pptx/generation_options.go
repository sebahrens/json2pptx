package main

// generationOptions adds optional CLI controls without changing the existing
// programmatic callers' defaults.
type generationOptions struct {
	FitReport, VerboseFit      bool
	PlaceholderPolicy, BaseDir string
}

func effectiveGenerationOptions(options []generationOptions) generationOptions {
	var out generationOptions
	if len(options) > 0 {
		out = options[0]
	}
	out.PlaceholderPolicy = normalizePlaceholderPolicy(out.PlaceholderPolicy)
	return out
}
