#include "simulator.h"

#include <iostream>
#include <string>

namespace {

void AddNode(ghost::v1::TopologySnapshot* topology, const std::string& name, bool tainted) {
    auto* node = topology->add_nodes();
    node->set_name(name);
    node->set_status("Ready");
    node->set_unschedulable(false);
    if (tainted) {
        node->add_taints("dedicated");
    }
    node->mutable_headroom()->set_cpu_milli(1000);
    node->mutable_headroom()->set_memory_bytes(1000);
}

ghost::v1::SimulationRequest BaseRequest() {
    ghost::v1::SimulationRequest request;
    request.set_action("node_drain");
    request.set_node_name("node-a");
    request.set_horizon_seconds(900);

    auto* topology = request.mutable_topology();
    AddNode(topology, "node-a", false);
    AddNode(topology, "node-b", true);
    AddNode(topology, "node-c", false);

    auto* pod = topology->add_pods();
    pod->set_id("default/api");
    pod->set_namespace_("default");
    pod->set_name("api");
    pod->set_node_name("node-a");
    pod->set_status("Running");
    pod->mutable_requests()->set_cpu_milli(100);
    pod->mutable_requests()->set_memory_bytes(100);

    return request;
}

std::string FinalNodeName(const ghost::v1::SimulationResult& result) {
    if (result.frames_size() == 0) {
        return "";
    }
    const auto& frame = result.frames(result.frames_size() - 1);
    for (const auto& pod : frame.pods()) {
        if (pod.id() == "default/api") {
            return pod.node_name();
        }
    }
    return "";
}

bool ExpectEqual(const std::string& label, const std::string& got, const std::string& want) {
    if (got == want) {
        return true;
    }
    std::cerr << label << ": got " << got << ", want " << want << "\n";
    return false;
}

} // namespace

int main() {
    auto untolerated = BaseRequest();
    auto untolerated_result = ghost::Simulator::Run(untolerated);
    if (!ExpectEqual("untolerated destination", FinalNodeName(untolerated_result), "node-c")) {
        return 1;
    }

    auto tolerated = BaseRequest();
    tolerated.mutable_topology()->mutable_pods(0)->add_tolerations("dedicated=:NoSchedule");
    auto tolerated_result = ghost::Simulator::Run(tolerated);
    if (!ExpectEqual("tolerated destination", FinalNodeName(tolerated_result), "node-b")) {
        return 1;
    }

    return 0;
}
