package earthfile

import "sync"

// Canonical command name variables to enable lightweight string interning.
// Returning these variables ensures all occurrences share the same backing data.
var (
	nameFROM           = "FROM"
	nameRUN            = "RUN"
	nameCOPY           = "COPY"
	nameBUILD          = "BUILD"
	nameARG            = "ARG"
	nameSAVEARTIFACT   = "SAVE ARTIFACT"
	nameSAVEIMAGE      = "SAVE IMAGE"
	nameCMD            = "CMD"
	nameENTRYPOINT     = "ENTRYPOINT"
	nameEXPOSE         = "EXPOSE"
	nameVOLUME         = "VOLUME"
	nameENV            = "ENV"
	nameWORKDIR        = "WORKDIR"
	nameUSER           = "USER"
	nameGITCLONE       = "GIT CLONE"
	nameADD            = "ADD"
	nameSTOPSIGNAL     = "STOPSIGNAL"
	nameONBUILD        = "ONBUILD"
	nameHEALTHCHECK    = "HEALTHCHECK"
	nameSHELL          = "SHELL"
	nameDO             = "DO"
	nameCOMMAND        = "COMMAND"
	nameIMPORT         = "IMPORT"
	nameVERSION        = "VERSION"
	nameFROMDOCKERFILE = "FROM DOCKERFILE"
	nameLOCALLY        = "LOCALLY"
	nameHOST           = "HOST"
	namePROJECT        = "PROJECT"
	nameCACHE          = "CACHE"
	nameSET            = "SET"
	nameLET            = "LET"
	nameTRY            = "TRY"
	nameWITH           = "WITH"
	nameIF             = "IF"
	nameELSEIF         = "ELSE IF"
	nameELSE           = "ELSE"
	nameFOR            = "FOR"
	nameWAIT           = "WAIT"
	nameEND            = "END"
	nameCATCH          = "CATCH"
	nameFINALLY        = "FINALLY"
)

var dynamicIntern sync.Map // map[string]string

var knownInterned = map[string]string{
	"FROM":            nameFROM,
	"RUN":             nameRUN,
	"COPY":            nameCOPY,
	"BUILD":           nameBUILD,
	"ARG":             nameARG,
	"SAVE ARTIFACT":   nameSAVEARTIFACT,
	"SAVE IMAGE":      nameSAVEIMAGE,
	"CMD":             nameCMD,
	"ENTRYPOINT":      nameENTRYPOINT,
	"EXPOSE":          nameEXPOSE,
	"VOLUME":          nameVOLUME,
	"ENV":             nameENV,
	"WORKDIR":         nameWORKDIR,
	"USER":            nameUSER,
	"GIT CLONE":       nameGITCLONE,
	"ADD":             nameADD,
	"STOPSIGNAL":      nameSTOPSIGNAL,
	"ONBUILD":         nameONBUILD,
	"HEALTHCHECK":     nameHEALTHCHECK,
	"SHELL":           nameSHELL,
	"DO":              nameDO,
	"COMMAND":         nameCOMMAND,
	"IMPORT":          nameIMPORT,
	"VERSION":         nameVERSION,
	"FROM DOCKERFILE": nameFROMDOCKERFILE,
	"LOCALLY":         nameLOCALLY,
	"HOST":            nameHOST,
	"PROJECT":         namePROJECT,
	"CACHE":           nameCACHE,
	"SET":             nameSET,
	"LET":             nameLET,
	"TRY":             nameTRY,
	"WITH":            nameWITH,
	"IF":              nameIF,
	"ELSE IF":         nameELSEIF,
	"ELSE":            nameELSE,
	"FOR":             nameFOR,
	"WAIT":            nameWAIT,
	"END":             nameEND,
	"CATCH":           nameCATCH,
	"FINALLY":         nameFINALLY,
}

// internCommandName returns a canonical string for a known command name.
// For unknown names, it dynamically interns using a concurrent map.
func internCommandName(name string) string {
	if v, ok := knownInterned[name]; ok {
		return v
	}
	if v, ok := dynamicIntern.Load(name); ok {
		return v.(string)
	}
	// Store a copy to ensure the interned instance does not reference a large backing array
	interned := ("" + name)
	dynamicIntern.Store(name, interned)
	return interned
}
