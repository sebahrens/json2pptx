package pptx

import "testing"

func TestPresetGeometry_Constants(t *testing.T) {
	t.Parallel()

	// Verify all 20 Phase 1 geometries have the expected string values.
	geoms := []struct {
		geom PresetGeometry
		want string
	}{
		{GeomRect, "rect"},
		{GeomRoundRect, "roundRect"},
		{GeomRound1Rect, "round1Rect"},
		{GeomRound2SameRect, "round2SameRect"},
		{GeomEllipse, "ellipse"},
		{GeomTriangle, "triangle"},
		{GeomDiamond, "diamond"},
		{GeomParallelogram, "parallelogram"},
		{GeomTrapezoid, "trapezoid"},
		{GeomHexagon, "hexagon"},
		{GeomOctagon, "octagon"},
		{GeomChevron, "chevron"},
		{GeomHomePlate, "homePlate"},
		{GeomPentagon, "pentagon"},
		{GeomPlus, "plus"},
		{GeomRightArrow, "rightArrow"},
		{GeomLeftArrow, "leftArrow"},
		{GeomUpArrow, "upArrow"},
		{GeomDownArrow, "downArrow"},
		{GeomDonut, "donut"},
	}

	for _, tt := range geoms {
		if string(tt.geom) != tt.want {
			t.Errorf("GeomConstant: got %q, want %q", tt.geom, tt.want)
		}
	}

	if len(geoms) != 20 {
		t.Errorf("expected 20 Phase 1 geometries, got %d", len(geoms))
	}
}

func TestPresetGeometry_Phase2Constants(t *testing.T) {
	t.Parallel()

	geoms := []struct {
		geom PresetGeometry
		want string
	}{
		// Flowchart shapes
		{GeomFlowChartProcess, "flowChartProcess"},
		{GeomFlowChartDecision, "flowChartDecision"},
		{GeomFlowChartTerminator, "flowChartTerminator"},
		{GeomFlowChartDocument, "flowChartDocument"},
		{GeomFlowChartMultidocument, "flowChartMultidocument"},
		{GeomFlowChartInputOutput, "flowChartInputOutput"},
		{GeomFlowChartPredefinedProcess, "flowChartPredefinedProcess"},
		{GeomFlowChartInternalStorage, "flowChartInternalStorage"},
		{GeomFlowChartPreparation, "flowChartPreparation"},
		{GeomFlowChartManualInput, "flowChartManualInput"},
		{GeomFlowChartManualOperation, "flowChartManualOperation"},
		{GeomFlowChartConnector, "flowChartConnector"},
		{GeomFlowChartOffpageConnector, "flowChartOffpageConnector"},
		{GeomFlowChartAlternateProcess, "flowChartAlternateProcess"},
		// Additional arrows
		{GeomLeftRightArrow, "leftRightArrow"},
		{GeomUpDownArrow, "upDownArrow"},
		{GeomNotchedRightArrow, "notchedRightArrow"},
		{GeomStripedRightArrow, "stripedRightArrow"},
		{GeomCurvedRightArrow, "curvedRightArrow"},
		{GeomCurvedLeftArrow, "curvedLeftArrow"},
		{GeomBentArrow, "bentArrow"},
		{GeomBentUpArrow, "bentUpArrow"},
	}

	for _, tt := range geoms {
		if string(tt.geom) != tt.want {
			t.Errorf("GeomConstant: got %q, want %q", tt.geom, tt.want)
		}
	}

	if len(geoms) != 22 {
		t.Errorf("expected 22 Phase 2 geometries, got %d", len(geoms))
	}
}

func TestPresetGeometry_Phase3Constants(t *testing.T) {
	t.Parallel()

	geoms := []struct {
		geom PresetGeometry
		want string
	}{
		// Callouts
		{GeomWedgeRectCallout, "wedgeRectCallout"},
		{GeomWedgeRoundRectCallout, "wedgeRoundRectCallout"},
		{GeomWedgeEllipseCallout, "wedgeEllipseCallout"},
		{GeomCloudCallout, "cloudCallout"},
		{GeomCloud, "cloud"},
		{GeomCallout1, "callout1"},
		{GeomCallout2, "callout2"},
		{GeomCallout3, "callout3"},
		{GeomBorderCallout1, "borderCallout1"},
		{GeomBorderCallout2, "borderCallout2"},
		{GeomBorderCallout3, "borderCallout3"},
		// Stars and banners
		{GeomStar4, "star4"},
		{GeomStar5, "star5"},
		{GeomStar6, "star6"},
		{GeomStar7, "star7"},
		{GeomStar8, "star8"},
		{GeomStar10, "star10"},
		{GeomStar12, "star12"},
		{GeomRibbon, "ribbon"},
		{GeomRibbon2, "ribbon2"},
		{GeomIrregularSeal1, "irregularSeal1"},
		{GeomIrregularSeal2, "irregularSeal2"},
		// Miscellaneous
		{GeomHeart, "heart"},
		{GeomLightningBolt, "lightningBolt"},
		{GeomSun, "sun"},
		{GeomMoon, "moon"},
		{GeomSmileyFace, "smileyFace"},
		{GeomCube, "cube"},
		{GeomCan, "can"},
		{GeomFoldedCorner, "foldedCorner"},
		{GeomFrame, "frame"},
		{GeomBevel, "bevel"},
	}

	for _, tt := range geoms {
		if string(tt.geom) != tt.want {
			t.Errorf("GeomConstant: got %q, want %q", tt.geom, tt.want)
		}
	}

	if len(geoms) != 32 {
		t.Errorf("expected 32 Phase 3 geometries, got %d", len(geoms))
	}
}

func TestPresetGeometry_Phase4Constants(t *testing.T) {
	t.Parallel()

	geoms := []struct {
		geom PresetGeometry
		want string
	}{
		{GeomLine, "line"},
		{GeomLineInv, "lineInv"},
		{GeomArc, "arc"},
		{GeomBlockArc, "blockArc"},
		{GeomChord, "chord"},
		// Bracket and brace shapes
		{GeomLeftBracket, "leftBracket"},
		{GeomRightBracket, "rightBracket"},
		{GeomLeftBrace, "leftBrace"},
		{GeomRightBrace, "rightBrace"},
		{GeomBracketPair, "bracketPair"},
		{GeomBracePair, "bracePair"},
		// Right triangle
		{GeomRightTriangle, "rtTriangle"},
		// Snipped-corner rectangles
		{GeomSnip1Rect, "snip1Rect"},
		{GeomSnip2SameRect, "snip2SameRect"},
		{GeomSnip2DiagRect, "snip2DiagRect"},
		{GeomSnipRoundRect, "snipRoundRect"},
	}

	for _, tt := range geoms {
		if string(tt.geom) != tt.want {
			t.Errorf("GeomConstant: got %q, want %q", tt.geom, tt.want)
		}
	}

	if len(geoms) != 16 {
		t.Errorf("expected 16 Phase 4 geometries, got %d", len(geoms))
	}
}

func TestPresetGeometry_Phase5Constants(t *testing.T) {
	t.Parallel()

	geoms := []struct {
		geom PresetGeometry
		want string
	}{
		// Additional basic shapes
		{GeomRound2DiagRect, "round2DiagRect"},
		{GeomNonIsoscelesTrapezoid, "nonIsoscelesTrapezoid"},
		{GeomHeptagon, "heptagon"},
		{GeomDecagon, "decagon"},
		{GeomDodecagon, "dodecagon"},
		{GeomCross, "cross"},
		{GeomHalfFrame, "halfFrame"},
		{GeomCorner, "corner"},
		{GeomDiagStripe, "diagStripe"},
		{GeomPlaque, "plaque"},
		{GeomNoSmoking, "noSmoking"},
		{GeomPie, "pie"},
		{GeomPieWedge, "pieWedge"},
		{GeomTeardrop, "teardrop"},
		{GeomWave, "wave"},
		{GeomDoubleWave, "doubleWave"},
		{GeomVerticalScroll, "verticalScroll"},
		{GeomHorizontalScroll, "horizontalScroll"},
		// Additional stars
		{GeomStar16, "star16"},
		{GeomStar24, "star24"},
		{GeomStar32, "star32"},
		// Additional arrows
		{GeomLeftUpArrow, "leftUpArrow"},
		{GeomLeftRightUpArrow, "leftRightUpArrow"},
		{GeomQuadArrow, "quadArrow"},
		{GeomCurvedUpArrow, "curvedUpArrow"},
		{GeomCurvedDownArrow, "curvedDownArrow"},
		{GeomUturnArrow, "uturnArrow"},
		{GeomCircularArrow, "circularArrow"},
		{GeomLeftCircularArrow, "leftCircularArrow"},
		{GeomLeftRightCircularArrow, "leftRightCircularArrow"},
		{GeomSwooshArrow, "swooshArrow"},
		// Arrow callouts
		{GeomRightArrowCallout, "rightArrowCallout"},
		{GeomLeftArrowCallout, "leftArrowCallout"},
		{GeomUpArrowCallout, "upArrowCallout"},
		{GeomDownArrowCallout, "downArrowCallout"},
		{GeomLeftRightArrowCallout, "leftRightArrowCallout"},
		{GeomUpDownArrowCallout, "upDownArrowCallout"},
		{GeomQuadArrowCallout, "quadArrowCallout"},
		// Accent callouts
		{GeomAccentCallout1, "accentCallout1"},
		{GeomAccentCallout2, "accentCallout2"},
		{GeomAccentCallout3, "accentCallout3"},
		{GeomAccentBorderCallout1, "accentBorderCallout1"},
		{GeomAccentBorderCallout2, "accentBorderCallout2"},
		{GeomAccentBorderCallout3, "accentBorderCallout3"},
		// Equation shapes
		{GeomMathPlus, "mathPlus"},
		{GeomMathMinus, "mathMinus"},
		{GeomMathMultiply, "mathMultiply"},
		{GeomMathDivide, "mathDivide"},
		{GeomMathEqual, "mathEqual"},
		{GeomMathNotEqual, "mathNotEqual"},
		// Additional flowchart shapes
		{GeomFlowChartCollate, "flowChartCollate"},
		{GeomFlowChartSort, "flowChartSort"},
		{GeomFlowChartExtract, "flowChartExtract"},
		{GeomFlowChartMerge, "flowChartMerge"},
		{GeomFlowChartOnlineStorage, "flowChartOnlineStorage"},
		{GeomFlowChartOfflineStorage, "flowChartOfflineStorage"},
		{GeomFlowChartMagneticTape, "flowChartMagneticTape"},
		{GeomFlowChartMagneticDisk, "flowChartMagneticDisk"},
		{GeomFlowChartMagneticDrum, "flowChartMagneticDrum"},
		{GeomFlowChartDisplay, "flowChartDisplay"},
		{GeomFlowChartDelay, "flowChartDelay"},
		{GeomFlowChartPunchedCard, "flowChartPunchedCard"},
		{GeomFlowChartPunchedTape, "flowChartPunchedTape"},
		{GeomFlowChartSummingJunction, "flowChartSummingJunction"},
		{GeomFlowChartOr, "flowChartOr"},
		// Action buttons
		{GeomActionButtonBlank, "actionButtonBlank"},
		{GeomActionButtonHome, "actionButtonHome"},
		{GeomActionButtonHelp, "actionButtonHelp"},
		{GeomActionButtonInformation, "actionButtonInformation"},
		{GeomActionButtonBackPrevious, "actionButtonBackPrevious"},
		{GeomActionButtonForwardNext, "actionButtonForwardNext"},
		{GeomActionButtonBeginning, "actionButtonBeginning"},
		{GeomActionButtonEnd, "actionButtonEnd"},
		{GeomActionButtonReturn, "actionButtonReturn"},
		{GeomActionButtonDocument, "actionButtonDocument"},
		{GeomActionButtonSound, "actionButtonSound"},
		{GeomActionButtonMovie, "actionButtonMovie"},
		// Chart and tab shapes
		{GeomChartX, "chartX"},
		{GeomChartStar, "chartStar"},
		{GeomChartPlus, "chartPlus"},
		{GeomCornerTabs, "cornerTabs"},
		{GeomSquareTabs, "squareTabs"},
		{GeomPlaqueTabs, "plaqueTabs"},
		// Gears and funnel
		{GeomGear6, "gear6"},
		{GeomGear9, "gear9"},
		{GeomFunnel, "funnel"},
	}

	for _, tt := range geoms {
		if string(tt.geom) != tt.want {
			t.Errorf("GeomConstant: got %q, want %q", tt.geom, tt.want)
		}
	}

	if len(geoms) != 86 {
		t.Errorf("expected 86 Phase 5 geometries, got %d", len(geoms))
	}
}

func TestDefaultAdjustHandles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		geom  PresetGeometry
		names []string
	}{
		{GeomRect, nil},          // no handles
		{GeomEllipse, nil},       // no handles
		{GeomRoundRect, []string{"adj"}},
		{GeomDonut, []string{"adj"}},
		{GeomRightArrow, []string{"adj1", "adj2"}},
		{GeomLeftArrow, []string{"adj1", "adj2"}},
		{GeomRound2SameRect, []string{"adj1", "adj2"}},
		// Phase 2: flowchart shapes (most have no handles)
		{GeomFlowChartProcess, nil},
		{GeomFlowChartDecision, nil},
		{GeomFlowChartConnector, nil},
		{GeomFlowChartAlternateProcess, []string{"adj"}},
		// Phase 2: arrows
		{GeomLeftRightArrow, []string{"adj1", "adj2"}},
		{GeomUpDownArrow, []string{"adj1", "adj2"}},
		{GeomCurvedRightArrow, []string{"adj1", "adj2", "adj3"}},
		{GeomBentArrow, []string{"adj1", "adj2", "adj3", "adj4"}},
		{GeomBentUpArrow, []string{"adj1", "adj2", "adj3"}},
		// Phase 3: callouts
		{GeomWedgeRectCallout, []string{"adj1", "adj2"}},
		{GeomWedgeRoundRectCallout, []string{"adj1", "adj2"}},
		{GeomWedgeEllipseCallout, []string{"adj1", "adj2"}},
		{GeomCloudCallout, []string{"adj1", "adj2"}},
		{GeomCloud, nil},
		{GeomCallout1, []string{"adj1", "adj2", "adj3", "adj4"}},
		{GeomCallout2, []string{"adj1", "adj2", "adj3", "adj4", "adj5", "adj6"}},
		{GeomCallout3, []string{"adj1", "adj2", "adj3", "adj4", "adj5", "adj6", "adj7", "adj8"}},
		{GeomBorderCallout1, []string{"adj1", "adj2", "adj3", "adj4"}},
		{GeomBorderCallout2, []string{"adj1", "adj2", "adj3", "adj4", "adj5", "adj6"}},
		{GeomBorderCallout3, []string{"adj1", "adj2", "adj3", "adj4", "adj5", "adj6", "adj7", "adj8"}},
		// Phase 3: stars (all have single adj)
		{GeomStar4, []string{"adj"}},
		{GeomStar5, []string{"adj"}},
		{GeomStar10, []string{"adj"}},
		{GeomIrregularSeal1, nil},
		{GeomIrregularSeal2, nil},
		// Phase 3: banners
		{GeomRibbon, []string{"adj1", "adj2"}},
		{GeomRibbon2, []string{"adj1", "adj2"}},
		// Phase 3: miscellaneous
		{GeomHeart, nil},
		{GeomLightningBolt, nil},
		{GeomSun, []string{"adj"}},
		{GeomMoon, []string{"adj"}},
		{GeomSmileyFace, []string{"adj"}},
		{GeomCube, []string{"adj"}},
		{GeomCan, []string{"adj"}},
		{GeomFoldedCorner, []string{"adj"}},
		{GeomFrame, []string{"adj1"}},
		{GeomBevel, []string{"adj"}},
		// Phase 4: line shapes (no handles)
		{GeomLine, nil},
		{GeomLineInv, nil},
		// Phase 4: arc shapes
		{GeomArc, []string{"adj1", "adj2"}},
		{GeomBlockArc, []string{"adj1", "adj2", "adj3"}},
		{GeomChord, []string{"adj1", "adj2"}},
		// Phase 4: bracket and brace shapes
		{GeomLeftBracket, []string{"adj"}},
		{GeomRightBracket, []string{"adj"}},
		{GeomLeftBrace, []string{"adj1", "adj2"}},
		{GeomRightBrace, []string{"adj1", "adj2"}},
		{GeomBracketPair, []string{"adj"}},
		{GeomBracePair, []string{"adj"}},
		// Phase 4: right triangle (no handles)
		{GeomRightTriangle, nil},
		// Phase 4: snipped-corner rectangles
		{GeomSnip1Rect, []string{"adj"}},
		{GeomSnip2SameRect, []string{"adj1", "adj2"}},
		{GeomSnip2DiagRect, []string{"adj1", "adj2"}},
		{GeomSnipRoundRect, []string{"adj1", "adj2"}},
		// Phase 5: additional basic shapes
		{GeomRound2DiagRect, []string{"adj1", "adj2"}},
		{GeomNonIsoscelesTrapezoid, []string{"adj1", "adj2"}},
		{GeomHeptagon, nil},
		{GeomDecagon, nil},
		{GeomDodecagon, nil},
		{GeomCross, []string{"adj"}},
		{GeomHalfFrame, []string{"adj1", "adj2"}},
		{GeomCorner, []string{"adj1", "adj2"}},
		{GeomDiagStripe, []string{"adj"}},
		{GeomPlaque, []string{"adj"}},
		{GeomNoSmoking, []string{"adj"}},
		{GeomPie, []string{"adj1", "adj2"}},
		{GeomPieWedge, nil},
		{GeomTeardrop, []string{"adj"}},
		{GeomWave, []string{"adj1", "adj2"}},
		{GeomDoubleWave, []string{"adj1", "adj2"}},
		{GeomVerticalScroll, []string{"adj"}},
		{GeomHorizontalScroll, []string{"adj"}},
		// Phase 5: additional stars
		{GeomStar16, []string{"adj"}},
		{GeomStar24, []string{"adj"}},
		{GeomStar32, []string{"adj"}},
		// Phase 5: additional arrows
		{GeomLeftUpArrow, []string{"adj1", "adj2", "adj3"}},
		{GeomLeftRightUpArrow, []string{"adj1", "adj2", "adj3"}},
		{GeomQuadArrow, []string{"adj1", "adj2", "adj3"}},
		{GeomCurvedUpArrow, []string{"adj1", "adj2", "adj3"}},
		{GeomCurvedDownArrow, []string{"adj1", "adj2", "adj3"}},
		{GeomUturnArrow, []string{"adj1", "adj2", "adj3", "adj4", "adj5"}},
		{GeomCircularArrow, []string{"adj1", "adj2", "adj3", "adj4", "adj5"}},
		{GeomLeftCircularArrow, []string{"adj1", "adj2", "adj3", "adj4", "adj5"}},
		{GeomLeftRightCircularArrow, []string{"adj1", "adj2", "adj3", "adj4", "adj5"}},
		{GeomSwooshArrow, []string{"adj1", "adj2"}},
		// Phase 5: arrow callouts
		{GeomRightArrowCallout, []string{"adj1", "adj2", "adj3", "adj4"}},
		{GeomLeftArrowCallout, []string{"adj1", "adj2", "adj3", "adj4"}},
		{GeomUpArrowCallout, []string{"adj1", "adj2", "adj3", "adj4"}},
		{GeomDownArrowCallout, []string{"adj1", "adj2", "adj3", "adj4"}},
		{GeomLeftRightArrowCallout, []string{"adj1", "adj2", "adj3", "adj4"}},
		{GeomUpDownArrowCallout, []string{"adj1", "adj2", "adj3", "adj4"}},
		{GeomQuadArrowCallout, []string{"adj1", "adj2", "adj3", "adj4"}},
		// Phase 5: accent callouts
		{GeomAccentCallout1, []string{"adj1", "adj2", "adj3", "adj4"}},
		{GeomAccentCallout2, []string{"adj1", "adj2", "adj3", "adj4", "adj5", "adj6"}},
		{GeomAccentCallout3, []string{"adj1", "adj2", "adj3", "adj4", "adj5", "adj6", "adj7", "adj8"}},
		{GeomAccentBorderCallout1, []string{"adj1", "adj2", "adj3", "adj4"}},
		{GeomAccentBorderCallout2, []string{"adj1", "adj2", "adj3", "adj4", "adj5", "adj6"}},
		{GeomAccentBorderCallout3, []string{"adj1", "adj2", "adj3", "adj4", "adj5", "adj6", "adj7", "adj8"}},
		// Phase 5: equation shapes
		{GeomMathPlus, []string{"adj"}},
		{GeomMathMinus, []string{"adj"}},
		{GeomMathMultiply, []string{"adj"}},
		{GeomMathDivide, []string{"adj1", "adj2"}},
		{GeomMathEqual, []string{"adj1", "adj2"}},
		{GeomMathNotEqual, []string{"adj1", "adj2", "adj3"}},
		// Phase 5: gears
		{GeomGear6, []string{"adj1", "adj2"}},
		{GeomGear9, []string{"adj1", "adj2"}},
		{GeomFunnel, nil},
		// Phase 5: flowchart (no handles)
		{GeomFlowChartCollate, nil},
		{GeomFlowChartSort, nil},
		{GeomFlowChartDisplay, nil},
		// Phase 5: action buttons (no handles)
		{GeomActionButtonBlank, nil},
		{GeomActionButtonHome, nil},
		// Phase 5: chart/tab shapes (no handles)
		{GeomChartX, nil},
		{GeomChartStar, nil},
	}

	for _, tt := range tests {
		t.Run(string(tt.geom), func(t *testing.T) {
			got := DefaultAdjustHandles(tt.geom)
			if tt.names == nil {
				if got != nil {
					t.Errorf("expected nil handles for %s, got %v", tt.geom, got)
				}
				return
			}
			if len(got) != len(tt.names) {
				t.Fatalf("expected %d handles, got %d", len(tt.names), len(got))
			}
			for i, name := range tt.names {
				if got[i] != name {
					t.Errorf("handle[%d]: got %q, want %q", i, got[i], name)
				}
			}
		})
	}
}
