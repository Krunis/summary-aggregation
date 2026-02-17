package aggreagationserver

import (
	"errors"

	"strings"



	pb "github.com/Krunis/summary-aggregation/packages/grpcapi"
)


func ValidateSummReq(req *pb.UserSummaryRequest) error {
    var errorSlice []string
    
    if strings.TrimSpace(req.Username) == "" {
        errorSlice = append(errorSlice, "username is required")
    }
    if strings.TrimSpace(req.UserSummaryName) == "" {
        errorSlice = append(errorSlice, "user_summary_name is required")
    }
    
    if len(errorSlice) > 0 {
        return errors.New(strings.Join(errorSlice, ", "))
    }
    return nil
}


