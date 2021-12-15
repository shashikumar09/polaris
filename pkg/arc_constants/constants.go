package arc_constants

import "os"

var MACHINE_STABILITY string = os.Getenv("MACHINE_STABILITY")
const DEV = "dev"
const QA = "qa"
const PROD = "prod"
const UAT = "uat"
