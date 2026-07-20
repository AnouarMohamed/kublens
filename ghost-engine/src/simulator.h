#pragma once

#include <string>
#include "ghost/v1/ghost.grpc.pb.h"

namespace ghost {

class Simulator {
public:
    static ghost::v1::SimulationResult Run(const ghost::v1::SimulationRequest& request);

private:
    static std::string GetCurrentRFC3339();
    static std::string ComputeSHA1(const std::string& input);
    static bool IEquals(const std::string& a, const std::string& b);
    static bool NodeSelectorMatches(
        const google::protobuf::Map<std::string, std::string>& selector,
        const google::protobuf::Map<std::string, std::string>& labels
    );
    static bool ToleratesNodeTaints(
        const google::protobuf::RepeatedPtrField<std::string>& tolerations,
        const google::protobuf::RepeatedPtrField<std::string>& taints
    );
    static std::string TaintKey(const std::string& value);
};

} // namespace ghost
