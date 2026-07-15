// Package parse provides two-pass XML decoding for AIXM messages.
package parse

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"sync"

	"gitlab.com/davidkohl/goaixm/features"
)

// FeatureFactory returns a pointer to a zero-value concrete feature struct.
type FeatureFactory func() any

// Registry maps XML local names to feature factory functions.
type Registry struct {
	mu        sync.RWMutex
	factories map[string]FeatureFactory
}

// Register adds a factory for the given XML local name.
func (r *Registry) Register(localName string, f FeatureFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.factories == nil {
		r.factories = make(map[string]FeatureFactory)
	}
	r.factories[localName] = f
}

// Decode finds the factory for localName and decodes data into a new instance.
// data must be a document whose root element is a namespace-wrapper (_root_)
// containing the feature element as its first child.
func (r *Registry) Decode(localName string, data []byte) (any, error) {
	r.mu.RLock()
	f, ok := r.factories[localName]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("parse: unknown feature type %q", localName)
	}
	v := f()
	d := xml.NewDecoder(bytes.NewReader(data))
	// Advance past the _root_ wrapper to the first child element.
	for {
		tok, err := d.Token()
		if err != nil {
			return nil, fmt.Errorf("parse: decode %s: %w", localName, err)
		}
		if se, ok := tok.(xml.StartElement); ok {
			if se.Name.Local == "_root_" {
				continue
			}
			// This is the feature element; decode it into v.
			if err := d.DecodeElement(v, &se); err != nil {
				return nil, fmt.Errorf("parse: decode %s: %w", localName, err)
			}
			return v, nil
		}
	}
}

// DefaultRegistry is the package-level feature registry populated by init().
var DefaultRegistry = &Registry{}

func init() {
	r := DefaultRegistry
	// Note: Abstract feature types (AbstractGroundLightSystem, AbstractMarking, etc.) are
	// not registered because they are never instantiated directly in AIXM messages.
	r.Register("AerialRefuelling", func() any { return new(features.AerialRefuellingType) })
	r.Register("AeronauticalGroundLight", func() any { return new(features.AeronauticalGroundLightType) })
	r.Register("AircraftStand", func() any { return new(features.AircraftStandType) })
	r.Register("AirportHeliport", func() any { return new(features.AirportHeliportType) })
	r.Register("AirportHeliportCollocation", func() any { return new(features.AirportHeliportCollocationType) })
	r.Register("AirportHotSpot", func() any { return new(features.AirportHotSpotType) })
	r.Register("Airspace", func() any { return new(features.AirspaceType) })
	r.Register("AirspaceBorderCrossing", func() any { return new(features.AirspaceBorderCrossingType) })
	r.Register("AltimeterSource", func() any { return new(features.AltimeterSourceType) })
	r.Register("AngleIndication", func() any { return new(features.AngleIndicationType) })
	r.Register("Apron", func() any { return new(features.ApronType) })
	r.Register("ApronElement", func() any { return new(features.ApronElementType) })
	r.Register("ArrestingGear", func() any { return new(features.ArrestingGearType) })
	r.Register("AuthorityForAirspace", func() any { return new(features.AuthorityForAirspaceType) })
	r.Register("ChangeOverPoint", func() any { return new(features.ChangeOverPointType) })
	r.Register("CirclingArea", func() any { return new(features.CirclingAreaType) })
	r.Register("DeicingArea", func() any { return new(features.DeicingAreaType) })
	r.Register("DesignatedPoint", func() any { return new(features.DesignatedPointType) })
	r.Register("DistanceIndication", func() any { return new(features.DistanceIndicationType) })
	r.Register("FlightRestriction", func() any { return new(features.FlightRestrictionType) })
	r.Register("FloatingDockSite", func() any { return new(features.FloatingDockSiteType) })
	r.Register("GeoBorder", func() any { return new(features.GeoBorderType) })
	r.Register("GuidanceLine", func() any { return new(features.GuidanceLineType) })
	r.Register("HoldingAssessment", func() any { return new(features.HoldingAssessmentType) })
	r.Register("HoldingPattern", func() any { return new(features.HoldingPatternType) })
	r.Register("MarkingBuoy", func() any { return new(features.MarkingBuoyType) })
	r.Register("Navaid", func() any { return new(features.NavaidType) })
	r.Register("NavigationArea", func() any { return new(features.NavigationAreaType) })
	r.Register("NavigationAreaRestriction", func() any { return new(features.NavigationAreaRestrictionType) })
	r.Register("NonMovementArea", func() any { return new(features.NonMovementAreaType) })
	r.Register("ObstacleArea", func() any { return new(features.ObstacleAreaType) })
	r.Register("OrganisationAuthority", func() any { return new(features.OrganisationAuthorityType) })
	r.Register("PassengerLoadingBridge", func() any { return new(features.PassengerLoadingBridgeType) })
	r.Register("PilotControlledLighting", func() any { return new(features.PilotControlledLightingType) })
	r.Register("ProcedureDME", func() any { return new(features.ProcedureDMEType) })
	r.Register("RadarSystem", func() any { return new(features.RadarSystemType) })
	r.Register("RadioCommunicationChannel", func() any { return new(features.RadioCommunicationChannelType) })
	r.Register("RadioFrequencyArea", func() any { return new(features.RadioFrequencyAreaType) })
	r.Register("Road", func() any { return new(features.RoadType) })
	r.Register("Route", func() any { return new(features.RouteType) })
	r.Register("RouteDME", func() any { return new(features.RouteDMEType) })
	r.Register("RouteSegment", func() any { return new(features.RouteSegmentType) })
	r.Register("RulesProcedures", func() any { return new(features.RulesProceduresType) })
	r.Register("Runway", func() any { return new(features.RunwayType) })
	r.Register("RunwayBlastPad", func() any { return new(features.RunwayBlastPadType) })
	r.Register("RunwayCentrelinePoint", func() any { return new(features.RunwayCentrelinePointType) })
	r.Register("RunwayDirection", func() any { return new(features.RunwayDirectionType) })
	r.Register("RunwayElement", func() any { return new(features.RunwayElementType) })
	r.Register("RunwayVisualRange", func() any { return new(features.RunwayVisualRangeType) })
	r.Register("SafeAltitudeArea", func() any { return new(features.SafeAltitudeAreaType) })
	r.Register("SeaplaneLandingArea", func() any { return new(features.SeaplaneLandingAreaType) })
	r.Register("SeaplaneRampSite", func() any { return new(features.SeaplaneRampSiteType) })
	r.Register("SignificantPointInAirspace", func() any { return new(features.SignificantPointInAirspaceType) })
	r.Register("SpecialDate", func() any { return new(features.SpecialDateType) })
	r.Register("SpecialNavigationStation", func() any { return new(features.SpecialNavigationStationType) })
	r.Register("SpecialNavigationSystem", func() any { return new(features.SpecialNavigationSystemType) })
	r.Register("StandardLevelColumn", func() any { return new(features.StandardLevelColumnType) })
	r.Register("StandardLevelSector", func() any { return new(features.StandardLevelSectorType) })
	r.Register("StandardLevelTable", func() any { return new(features.StandardLevelTableType) })
	r.Register("SurveyControlPoint", func() any { return new(features.SurveyControlPointType) })
	r.Register("TaxiHoldingPosition", func() any { return new(features.TaxiHoldingPositionType) })
	r.Register("Taxiway", func() any { return new(features.TaxiwayType) })
	r.Register("TaxiwayElement", func() any { return new(features.TaxiwayElementType) })
	r.Register("TerminalArrivalArea", func() any { return new(features.TerminalArrivalAreaType) })
	r.Register("TouchDownLiftOff", func() any { return new(features.TouchDownLiftOffType) })
	r.Register("Unit", func() any { return new(features.UnitType) })
	r.Register("UnplannedHolding", func() any { return new(features.UnplannedHoldingType) })
	r.Register("VerticalStructure", func() any { return new(features.VerticalStructureType) })
	r.Register("WorkArea", func() any { return new(features.WorkAreaType) })
	// Indirect (abstract subtype) features
	r.Register("AirTrafficControlService", func() any { return new(features.AirTrafficControlServiceType) })
	r.Register("AirTrafficManagementService", func() any { return new(features.AirTrafficManagementServiceType) })
	r.Register("AircraftGroundService", func() any { return new(features.AircraftGroundServiceType) })
	r.Register("AirportClearanceService", func() any { return new(features.AirportClearanceServiceType) })
	r.Register("AirportProtectionAreaMarking", func() any { return new(features.AirportProtectionAreaMarkingType) })
	r.Register("AirportSuppliesService", func() any { return new(features.AirportSuppliesServiceType) })
	r.Register("ApproachLightingSystem", func() any { return new(features.ApproachLightingSystemType) })
	r.Register("ApronLightSystem", func() any { return new(features.ApronLightSystemType) })
	r.Register("ApronMarking", func() any { return new(features.ApronMarkingType) })
	r.Register("ArrivalFeederLeg", func() any { return new(features.ArrivalFeederLegType) })
	r.Register("ArrivalLeg", func() any { return new(features.ArrivalLegType) })
	r.Register("Azimuth", func() any { return new(features.AzimuthType) })
	r.Register("CheckpointINS", func() any { return new(features.CheckpointINSType) })
	r.Register("CheckpointVOR", func() any { return new(features.CheckpointVORType) })
	r.Register("DME", func() any { return new(features.DMEType) })
	r.Register("DeicingAreaMarking", func() any { return new(features.DeicingAreaMarkingType) })
	r.Register("DepartureLeg", func() any { return new(features.DepartureLegType) })
	r.Register("DirectionFinder", func() any { return new(features.DirectionFinderType) })
	r.Register("Elevation", func() any { return new(features.ElevationType) })
	r.Register("FinalLeg", func() any { return new(features.FinalLegType) })
	r.Register("FireFightingService", func() any { return new(features.FireFightingServiceType) })
	r.Register("Glidepath", func() any { return new(features.GlidepathType) })
	r.Register("GroundTrafficControlService", func() any { return new(features.GroundTrafficControlServiceType) })
	r.Register("GuidanceLineLightSystem", func() any { return new(features.GuidanceLineLightSystemType) })
	r.Register("GuidanceLineMarking", func() any { return new(features.GuidanceLineMarkingType) })
	r.Register("InformationService", func() any { return new(features.InformationServiceType) })
	r.Register("InitialLeg", func() any { return new(features.InitialLegType) })
	r.Register("InstrumentApproachProcedure", func() any { return new(features.InstrumentApproachProcedureType) })
	r.Register("IntermediateLeg", func() any { return new(features.IntermediateLegType) })
	r.Register("Localizer", func() any { return new(features.LocalizerType) })
	r.Register("MarkerBeacon", func() any { return new(features.MarkerBeaconType) })
	r.Register("MissedApproachLeg", func() any { return new(features.MissedApproachLegType) })
	r.Register("NDB", func() any { return new(features.NDBType) })
	r.Register("PassengerService", func() any { return new(features.PassengerServiceType) })
	r.Register("PrecisionApproachRadar", func() any { return new(features.PrecisionApproachRadarType) })
	r.Register("PrimaryRadarSurveillance", func() any { return new(features.PrimarySurveillanceRadarType) })
	r.Register("RunwayDirectionLightSystem", func() any { return new(features.RunwayDirectionLightSystemType) })
	r.Register("RunwayMarking", func() any { return new(features.RunwayMarkingType) })
	r.Register("RunwayProtectAreaLightSystem", func() any { return new(features.RunwayProtectAreaLightSystemType) })
	r.Register("RunwayProtectArea", func() any { return new(features.RunwayProtectAreaType) })
	r.Register("SDF", func() any { return new(features.SDFType) })
	r.Register("SearchRescueService", func() any { return new(features.SearchRescueServiceType) })
	r.Register("SecondaryRadarSurveillance", func() any { return new(features.SecondarySurveillanceRadarType) })
	r.Register("StandMarking", func() any { return new(features.StandMarkingType) })
	r.Register("StandardInstrumentArrival", func() any { return new(features.StandardInstrumentArrivalType) })
	r.Register("StandardInstrumentDeparture", func() any { return new(features.StandardInstrumentDepartureType) })
	r.Register("TACAN", func() any { return new(features.TACANType) })
	r.Register("TaxiHoldingPositionLightSystem", func() any { return new(features.TaxiHoldingPositionLightSystemType) })
	r.Register("TaxiHoldingPositionMarking", func() any { return new(features.TaxiHoldingPositionMarkingType) })
	r.Register("TaxiwayLightSystem", func() any { return new(features.TaxiwayLightSystemType) })
	r.Register("TaxiwayMarking", func() any { return new(features.TaxiwayMarkingType) })
	r.Register("TouchDownLiftOffLightSystem", func() any { return new(features.TouchDownLiftOffLightSystemType) })
	r.Register("TouchDownLiftOffMarking", func() any { return new(features.TouchDownLiftOffMarkingType) })
	r.Register("TouchDownLiftOffSafeArea", func() any { return new(features.TouchDownLiftOffSafeAreaType) })
	r.Register("VOR", func() any { return new(features.VORType) })
	r.Register("VisualGlideSlopeIndicator", func() any { return new(features.VisualGlideSlopeIndicatorType) })
}
