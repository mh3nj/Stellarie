package idea

// chaosBank is Chaos Mode's fixed pool of challenges (spec section 36).
// Deliberately static rather than generated: a genuinely novel,
// never-repeating challenge generator is exactly the kind of open-ended
// creative task section 37 reserves for an OPTIONAL future local-AI
// layer, not something this AI-free phase should fake with combinatorial
// noise dressed up as creativity. What keeps this from feeling stale
// despite being a fixed list: pick.go excludes anything in the person's
// own recent chaos_history before picking (section 36 - "maintain a
// history so it doesn't repeatedly generate the same challenges"), and
// the bank itself is large and varied enough that a full cycle takes a
// long time to repeat.
//
// Several of these are the spec's own section 36 examples verbatim
// (they're this document's own suggested content, not third-party
// material); the rest extend that same spirit to fill out the pool
// across every intent and a spread of time budgets.
// ChaosBank returns Chaos Mode's challenge pool (see the var doc
// comment above) - exported so services.IdeaService can pass it to
// PickSuggestions without this package needing to expose the whole
// chaosBank variable itself for mutation.
func ChaosBank() []Candidate {
	return chaosBank
}

var chaosBank = []Candidate{
	{Kind: KindChaos, Title: "Make a logo for a fictional underwater postal service", EstimatedMinutes: 30, Tags: []Intent{IntentCreative, IntentFun}},
	{Kind: KindChaos, Title: "Build a useless but beautiful 30-minute desktop application", EstimatedMinutes: 30, Tags: []Intent{IntentCreative, IntentFun}},
	{Kind: KindChaos, Title: "Improve an abandoned project", EstimatedMinutes: 45, Tags: []Intent{IntentProductive}},
	{Kind: KindChaos, Title: "Build something using a technology you haven't touched recently", EstimatedMinutes: 60, Tags: []Intent{IntentLearn, IntentCreative}},
	{Kind: KindChaos, Title: "Make a beat using four sounds", EstimatedMinutes: 30, Tags: []Intent{IntentCreative, IntentFun}},
	{Kind: KindChaos, Title: "Design an interface with one unusual constraint", EstimatedMinutes: 30, Tags: []Intent{IntentCreative, IntentLearn}},
	{Kind: KindChaos, Title: "Write a README for a project that doesn't exist yet", EstimatedMinutes: 15, Tags: []Intent{IntentCreative, IntentProductive}},
	{Kind: KindChaos, Title: "Refactor the ugliest function you can find in ten minutes", EstimatedMinutes: 15, Tags: []Intent{IntentProductive, IntentLearn}},
	{Kind: KindChaos, Title: "Recreate a childhood favorite game mechanic in miniature", EstimatedMinutes: 60, Tags: []Intent{IntentFun, IntentCreative}},
	{Kind: KindChaos, Title: "Teach yourself one new keyboard shortcut and use it ten times on purpose", EstimatedMinutes: 15, Tags: []Intent{IntentLearn}},
	{Kind: KindChaos, Title: "Build a tiny tool that solves a five-minute annoyance in your own workflow", EstimatedMinutes: 45, Tags: []Intent{IntentProductive, IntentCreative}},
	{Kind: KindChaos, Title: "Sketch three logo concepts for a made-up mountain-climbing cheese brand", EstimatedMinutes: 20, Tags: []Intent{IntentCreative, IntentFun}},
	{Kind: KindChaos, Title: "Read one technical article outside your usual area and summarize it in five bullet points", EstimatedMinutes: 20, Tags: []Intent{IntentLearn}},
	{Kind: KindChaos, Title: "Give an old UI a five-minute face-lift using only spacing and typography", EstimatedMinutes: 20, Tags: []Intent{IntentCreative, IntentProductive}},
	{Kind: KindChaos, Title: "Prototype the weirdest feature idea you've been sitting on, no matter how impractical", EstimatedMinutes: 45, Tags: []Intent{IntentCreative, IntentFun}},
	{Kind: KindChaos, Title: "Write down every distraction that pulled you away this week, then pick the worst one to fix", EstimatedMinutes: 15, Tags: []Intent{IntentProductive}},
	{Kind: KindChaos, Title: "Build a one-screen app that does exactly one silly thing well", EstimatedMinutes: 60, Tags: []Intent{IntentFun, IntentCreative}},
	{Kind: KindChaos, Title: "Explain a project you're working on as if to a curious ten-year-old, in writing", EstimatedMinutes: 15, Tags: []Intent{IntentLearn, IntentCreative}},
	{Kind: KindChaos, Title: "Pick a color palette from a photo on your desktop and build something with it", EstimatedMinutes: 30, Tags: []Intent{IntentCreative}},
	{Kind: KindChaos, Title: "Set a 20-minute timer and see how far you can get on the task you've been avoiding", EstimatedMinutes: 20, Tags: []Intent{IntentProductive}},
	{Kind: KindChaos, Title: "Compose a 30-second piece of music with an instrument you don't normally use", EstimatedMinutes: 30, Tags: []Intent{IntentCreative, IntentFun}},
	{Kind: KindChaos, Title: "Write the worst possible version of your current project's pitch, then the best", EstimatedMinutes: 15, Tags: []Intent{IntentCreative, IntentFun}},
	{Kind: KindChaos, Title: "Learn just enough of a tool you've never opened to do one small real task in it", EstimatedMinutes: 30, Tags: []Intent{IntentLearn}},
	{Kind: KindChaos, Title: "Build something completely new - no plan, just start", EstimatedMinutes: 30, Tags: []Intent{IntentCreative, IntentFun, IntentSurprise}},
}
