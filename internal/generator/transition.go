// Package generator provides PPTX file generation from slide specifications.
package generator

import (
	"bytes"
	"strings"
)

// Supported OOXML transition types.
var validTransitions = map[string]bool{
	"fade":     true,
	"push":     true,
	"wipe":     true,
	"cover":    true,
	"uncover":  true,
	"cut":      true,
	"dissolve": true,
}

// IsValidTransition reports whether the given transition name is supported.
func IsValidTransition(name string) bool {
	return validTransitions[strings.ToLower(name)]
}

// normalizeTransitionSpeed maps user-friendly speed names to OOXML spd attribute values.
func normalizeTransitionSpeed(speed string) string {
	switch strings.ToLower(speed) {
	case "slow":
		return "slow"
	case "fast":
		return "fast"
	case "med", "medium", "":
		return "med"
	default:
		return "med"
	}
}

// buildTransitionXML generates a <p:transition> element string for the given transition type and speed.
// Returns empty string if the transition type is "none" or unrecognized.
func buildTransitionXML(transType, speed string) string {
	transType = strings.ToLower(transType)
	if transType == "" || transType == "none" {
		return ""
	}
	if !validTransitions[transType] {
		return ""
	}

	spdAttr := normalizeTransitionSpeed(speed)

	// Build the inner transition element based on type
	var inner string
	switch transType {
	case "fade":
		inner = `<p:fade/>`
	case "push":
		inner = `<p:push dir="l"/>`
	case "wipe":
		inner = `<p:wipe dir="d"/>`
	case "cover":
		inner = `<p:cover dir="l"/>`
	case "uncover":
		inner = `<p:uncover dir="l"/>`
	case "cut":
		inner = `<p:cut/>`
	case "dissolve":
		inner = `<p:dissolve/>`
	default:
		return ""
	}

	return `<p:transition spd="` + spdAttr + `">` + inner + `</p:transition>`
}

// buildBulletBuildTimingXML generates a <p:timing> element that makes bullet paragraphs
// appear one-by-one on click. This uses the OOXML "afterEffect" build animation model.
// bodyShapeID is the shape ID of the body placeholder containing the bullets.
func buildBulletBuildTimingXML() string {
	// This timing structure tells PowerPoint to build the slide content by paragraph.
	// The key elements:
	// - <p:bldLst> with <p:bldP> to declare paragraph-level build on a shape
	// - <p:tnLst> with the root timing node
	//
	// The spId="2" targets the body placeholder (which typically has id="3" in OOXML
	// but the build references use the shape index). We use a convention placeholder
	// that PowerPoint resolves by the build list reference.
	//
	// Note: This is a simplified version. Full animation control requires animMotion,
	// animEffect, etc. The paragraph build is the most common and useful animation.
	return `<p:timing>
  <p:tnLst>
    <p:par>
      <p:cTn id="1" dur="indefinite" restart="never" nodeType="tmRoot">
        <p:childTnLst>
          <p:seq concurrent="1" nextAc="seek">
            <p:cTn id="2" dur="indefinite" nodeType="mainSeq">
              <p:childTnLst>
                <p:par>
                  <p:cTn id="3" fill="hold">
                    <p:stCondLst>
                      <p:cond delay="0"/>
                    </p:stCondLst>
                    <p:childTnLst>
                      <p:par>
                        <p:cTn id="4" fill="hold">
                          <p:stCondLst>
                            <p:cond delay="0"/>
                          </p:stCondLst>
                          <p:childTnLst>
                            <p:par>
                              <p:cTn id="5" presetID="1" presetClass="entr" presetSubtype="0" fill="hold" nodeType="afterEffect">
                                <p:stCondLst>
                                  <p:cond delay="0"/>
                                </p:stCondLst>
                                <p:childTnLst>
                                  <p:set>
                                    <p:cBhvr>
                                      <p:cTn id="6" dur="1" fill="hold">
                                        <p:stCondLst>
                                          <p:cond delay="0"/>
                                        </p:stCondLst>
                                      </p:cTn>
                                      <p:tgtEl>
                                        <p:spTgt spId="3">
                                          <p:txEl>
                                            <p:pRg st="0" end="0"/>
                                          </p:txEl>
                                        </p:spTgt>
                                      </p:tgtEl>
                                      <p:attrNameLst>
                                        <p:attrName>style.visibility</p:attrName>
                                      </p:attrNameLst>
                                    </p:cBhvr>
                                    <p:to>
                                      <p:strVal val="visible"/>
                                    </p:to>
                                  </p:set>
                                </p:childTnLst>
                              </p:cTn>
                            </p:par>
                          </p:childTnLst>
                        </p:cTn>
                      </p:par>
                    </p:childTnLst>
                  </p:cTn>
                </p:par>
              </p:childTnLst>
            </p:cTn>
            <p:prevCondLst>
              <p:cond evt="onPrev" delay="0">
                <p:tgtEl>
                  <p:sldTgt/>
                </p:tgtEl>
              </p:cond>
            </p:prevCondLst>
            <p:nextCondLst>
              <p:cond evt="onNext" delay="0">
                <p:tgtEl>
                  <p:sldTgt/>
                </p:tgtEl>
              </p:cond>
            </p:nextCondLst>
          </p:seq>
        </p:childTnLst>
      </p:cTn>
    </p:par>
  </p:tnLst>
  <p:bldLst>
    <p:bldP spId="3" grpId="0" build="p"/>
  </p:bldLst>
</p:timing>`
}

// insertTransitionAndBuild inserts <p:transition> and <p:timing> elements into slide XML.
// These elements go after </p:cSld> and before <p:clrMapOvr> in the OOXML slide structure.
func insertTransitionAndBuild(slideData []byte, transition, speed, build string) []byte {
	transXML := buildTransitionXML(transition, speed)
	var buildXML string
	if strings.ToLower(build) == "bullets" {
		buildXML = buildBulletBuildTimingXML()
	}

	if transXML == "" && buildXML == "" {
		return slideData
	}

	// Insert before <p:clrMapOvr> (which comes after </p:cSld>)
	insertPoint := bytes.Index(slideData, []byte("<p:clrMapOvr"))
	if insertPoint == -1 {
		// Fallback: insert before </p:sld>
		insertPoint = bytes.LastIndex(slideData, []byte("</p:sld>"))
		if insertPoint == -1 {
			return slideData // Can't find insertion point
		}
	}

	var insertion string
	if transXML != "" {
		insertion += "\n" + transXML
	}
	if buildXML != "" {
		insertion += "\n" + buildXML
	}
	insertion += "\n"

	result := make([]byte, 0, len(slideData)+len(insertion))
	result = append(result, slideData[:insertPoint]...)
	result = append(result, insertion...)
	result = append(result, slideData[insertPoint:]...)
	return result
}
