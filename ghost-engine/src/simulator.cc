#include "simulator.h"
#include <chrono>
#include <iomanip>
#include <sstream>
#include <algorithm>
#include <cctype>
#include <iostream>

namespace ghost {

std::string Simulator::GetCurrentRFC3339() {
    auto now = std::chrono::system_clock::now();
    std::time_t now_time = std::chrono::system_clock::to_time_t(now);
    std::tm utc_tm = *std::gmtime(&now_time);
    char buf[30];
    std::strftime(buf, sizeof(buf), "%Y-%m-%dT%H:%M:%SZ", &utc_tm);
    return std::string(buf);
}

std::string Simulator::ComputeSHA1(const std::string& input) {
    uint32_t h0 = 0x67452301;
    uint32_t h1 = 0xEFCDAB89;
    uint32_t h2 = 0x98BADCFE;
    uint32_t h3 = 0x10325476;
    uint32_t h4 = 0xC3D2E1F0;

    std::string padded = input;
    uint64_t bit_len = padded.size() * 8;
    padded.push_back((char)0x80);
    while ((padded.size() % 64) != 56) {
        padded.push_back(0x00);
    }
    for (int i = 7; i >= 0; --i) {
        padded.push_back((char)((bit_len >> (i * 8)) & 0xFF));
    }

    auto left_rotate = [](uint32_t value, uint32_t shift) {
        return (value << shift) | (value >> (32 - shift));
    };

    for (size_t chunk = 0; chunk < padded.size(); chunk += 64) {
        uint32_t w[80] = {0};
        for (int i = 0; i < 16; ++i) {
            w[i] = ((uint8_t)padded[chunk + i * 4] << 24) |
                   ((uint8_t)padded[chunk + i * 4 + 1] << 16) |
                   ((uint8_t)padded[chunk + i * 4 + 2] << 8) |
                   ((uint8_t)padded[chunk + i * 4 + 3]);
        }
        for (int i = 16; i < 80; ++i) {
            w[i] = left_rotate(w[i - 3] ^ w[i - 8] ^ w[i - 14] ^ w[i - 16], 1);
        }

        uint32_t a = h0;
        uint32_t b = h1;
        uint32_t c = h2;
        uint32_t d = h3;
        uint32_t e = h4;

        for (int i = 0; i < 80; ++i) {
            uint32_t f, k;
            if (i < 20) {
                f = (b & c) | ((~b) & d);
                k = 0x5A827999;
            } else if (i < 40) {
                f = b ^ c ^ d;
                k = 0x6ED9EBA1;
            } else if (i < 60) {
                f = (b & c) | (b & d) | (c & d);
                k = 0x8F1BBCDC;
            } else {
                f = b ^ c ^ d;
                k = 0xCA62C1D6;
            }

            uint32_t temp = left_rotate(a, 5) + f + e + k + w[i];
            e = d;
            d = c;
            c = left_rotate(b, 30);
            b = a;
            a = temp;
        }

        h0 += a;
        h1 += b;
        h2 += c;
        h3 += d;
        h4 += e;
    }

    std::stringstream ss;
    ss << std::hex << std::setfill('0')
       << std::setw(8) << h0
       << std::setw(8) << h1
       << std::setw(8) << h2
       << std::setw(8) << h3
       << std::setw(8) << h4;
    return ss.str();
}

bool Simulator::IEquals(const std::string& a, const std::string& b) {
    if (a.size() != b.size()) return false;
    return std::equal(a.begin(), a.end(), b.begin(), [](char c1, char c2) {
        return std::tolower(static_cast<unsigned char>(c1)) == std::tolower(static_cast<unsigned char>(c2));
    });
}

bool Simulator::NodeSelectorMatches(
    const google::protobuf::Map<std::string, std::string>& selector,
    const google::protobuf::Map<std::string, std::string>& labels
) {
    for (auto const& [key, val] : selector) {
        auto it = labels.find(key);
        if (it == labels.end() || it->second != val) {
            return false;
        }
    }
    return true;
}

ghost::v1::SimulationResult Simulator::Run(const ghost::v1::SimulationRequest& request) {
    ghost::v1::SimulationResult result;
    
    // 1. Normalize request params
    std::string action = request.action();
    if (action.empty()) {
        action = "node_drain";
    }
    
    std::string node_name = request.node_name();
    
    int32_t horizon = request.horizon_seconds();
    if (horizon <= 0) {
        horizon = 900;
    } else if (horizon > 1800) {
        horizon = 1800;
    }
    
    std::string generated_at = GetCurrentRFC3339();
    
    result.set_action(action);
    result.set_generated_at(generated_at);
    result.set_horizon_seconds(horizon);
    
    // Calculate SHA1 Hash for Simulation ID
    std::string hash_input = action + "|" + node_name + "|" + generated_at;
    std::string sha = ComputeSHA1(hash_input);
    result.set_id("ghost-" + sha.substr(0, 12));
    
    // Copy topology to initial & target states
    const auto& initial_topology = request.topology();
    
    // 2. Find target node in topology
    bool node_found = false;
    for (int i = 0; i < initial_topology.nodes_size(); ++i) {
        if (initial_topology.nodes(i).name() == node_name) {
            node_found = true;
            break;
        }
    }
    
    if (!node_found) {
        // Target node not found: immediately return critical verdict
        auto* verdict = result.mutable_verdict();
        verdict->set_severity("critical");
        verdict->set_summary("Target node was not found in the topology.");
        verdict->add_recommendations("Refresh cluster state and rerun the simulation.");
        
        // Frame 0
        auto* frame0 = result.add_frames();
        frame0->set_offset_seconds(0);
        
        // Add nodes
        std::vector<ghost::v1::FrameNode> f0_nodes;
        for (int i = 0; i < initial_topology.nodes_size(); ++i) {
            const auto& n = initial_topology.nodes(i);
            ghost::v1::FrameNode fn;
            fn.set_name(n.name());
            fn.set_status(n.status());
            fn.set_unschedulable(n.unschedulable());
            *fn.mutable_headroom() = n.headroom();
            f0_nodes.push_back(fn);
        }
        std::sort(f0_nodes.begin(), f0_nodes.end(), [](const ghost::v1::FrameNode& a, const ghost::v1::FrameNode& b) {
            return a.name() < b.name();
        });
        for (const auto& fn : f0_nodes) {
            *frame0->add_nodes() = fn;
        }
        
        // Add pods
        std::vector<ghost::v1::FramePod> f0_pods;
        for (int i = 0; i < initial_topology.pods_size(); ++i) {
            const auto& p = initial_topology.pods(i);
            ghost::v1::FramePod fp;
            fp.set_id(p.id());
            fp.set_namespace_(p.namespace_());
            fp.set_name(p.name());
            fp.set_node_name(p.node_name());
            fp.set_status(p.status());
            f0_pods.push_back(fp);
        }
        std::sort(f0_pods.begin(), f0_pods.end(), [](const ghost::v1::FramePod& a, const ghost::v1::FramePod& b) {
            return a.id() < b.id();
        });
        for (const auto& fp : f0_pods) {
            *frame0->add_pods() = fp;
        }
        
        // Add critical event
        auto* ev = frame0->add_events();
        ev->set_kind("blocked");
        ev->set_severity("critical");
        ev->set_resource(node_name);
        ev->set_message("Target node does not exist.");
        ev->set_timestamp(generated_at);
        
        return result;
    }
    
    // Copy nodes & pods for simulation run
    std::vector<ghost::v1::Node> nodes_state;
    for (int i = 0; i < initial_topology.nodes_size(); ++i) {
        nodes_state.push_back(initial_topology.nodes(i));
    }
    
    std::vector<ghost::v1::Pod> pods_state;
    for (int i = 0; i < initial_topology.pods_size(); ++i) {
        pods_state.push_back(initial_topology.pods(i));
    }
    
    // Target node is selected by matching name in each generated frame.
    // Cordon the node
    for (auto& n : nodes_state) {
        if (n.name() == node_name) {
            n.set_unschedulable(true);
            break;
        }
    }
    
    std::vector<ghost::v1::TimelineEvent> simulation_events;
    
    // Cordoning event
    ghost::v1::TimelineEvent cordon_ev;
    cordon_ev.set_kind("node_cordoned");
    cordon_ev.set_severity("info");
    cordon_ev.set_resource(node_name);
    cordon_ev.set_message("Simulation marks " + node_name + " unschedulable before eviction.");
    cordon_ev.set_timestamp(generated_at);
    simulation_events.push_back(cordon_ev);
    
    // Sort pods by ID for deterministic processing
    std::sort(pods_state.begin(), pods_state.end(), [](const ghost::v1::Pod& a, const ghost::v1::Pod& b) {
        return a.id() < b.id();
    });
    
    int moved_count = 0;
    int unresolved_count = 0;
    
    for (auto& pod : pods_state) {
        if (pod.node_name() != node_name) {
            continue;
        }
        
        // Find candidate destination nodes
        std::vector<ghost::v1::Node*> candidates;
        for (auto& node : nodes_state) {
            if (node.name() == node_name) continue;
            if (node.unschedulable()) continue;
            if (!IEquals(node.status(), "Ready")) continue;
            if (!NodeSelectorMatches(pod.node_selector(), node.labels())) continue;
            
            // Resource checks
            if (node.headroom().cpu_milli() < pod.requests().cpu_milli()) continue;
            if (node.headroom().memory_bytes() < pod.requests().memory_bytes()) continue;
            
            candidates.push_back(&node);
        }
        
        if (candidates.empty()) {
            // No node found: pod becomes pending
            pod.set_status("Pending");
            pod.set_node_name("");
            unresolved_count++;
            
            ghost::v1::TimelineEvent pending_ev;
            pending_ev.set_kind("pod_pending");
            pending_ev.set_severity("critical");
            pending_ev.set_resource(pod.id());
            pending_ev.set_message(pod.id() + " cannot be placed after draining " + node_name + ".");
            pending_ev.set_timestamp(generated_at);
            simulation_events.push_back(pending_ev);
        } else {
            // Sort candidate nodes alphabetically by name
            std::sort(candidates.begin(), candidates.end(), [](const ghost::v1::Node* a, const ghost::v1::Node* b) {
                return a->name() < b->name();
            });
            
            ghost::v1::Node* dest = candidates[0];
            
            // Subtract resources from headroom
            auto* headroom = dest->mutable_headroom();
            headroom->set_cpu_milli(headroom->cpu_milli() - pod.requests().cpu_milli());
            headroom->set_memory_bytes(headroom->memory_bytes() - pod.requests().memory_bytes());
            
            pod.set_node_name(dest->name());
            moved_count++;
            
            ghost::v1::TimelineEvent rescheduled_ev;
            rescheduled_ev.set_kind("pod_rescheduled");
            rescheduled_ev.set_severity("info");
            rescheduled_ev.set_resource(pod.id());
            rescheduled_ev.set_message(pod.id() + " moves from " + node_name + " to " + dest->name() + ".");
            rescheduled_ev.set_timestamp(generated_at);
            simulation_events.push_back(rescheduled_ev);
        }
    }
    
    // Construct Verdict
    auto* verdict = result.mutable_verdict();
    if (unresolved_count > 0) {
        verdict->set_severity("critical");
        verdict->set_summary("Drain simulation leaves " + std::to_string(unresolved_count) + " pod(s) pending.");
        verdict->add_recommendations("Add capacity or relax scheduling constraints before draining this node.");
        verdict->add_recommendations("Run a live drain preview to compare Kubernetes eviction blockers.");
    } else if (moved_count > 0) {
        verdict->set_severity("warning");
        verdict->set_summary("Drain simulation can move " + std::to_string(moved_count) + " pod(s) from " + node_name + ".");
        verdict->add_recommendations("Review the simulated placements before running the real drain.");
        verdict->add_recommendations("Watch destination node headroom during the maintenance window.");
    } else {
        verdict->set_severity("info");
        verdict->set_summary("Drain simulation can move 0 pod(s) from " + node_name + ".");
        verdict->add_recommendations("Review the simulated placements before running the real drain.");
    }
    
    // Frame 0 (initial topology)
    auto* frame0 = result.add_frames();
    frame0->set_offset_seconds(0);
    
    std::vector<ghost::v1::FrameNode> f0_nodes;
    for (int i = 0; i < initial_topology.nodes_size(); ++i) {
        const auto& n = initial_topology.nodes(i);
        ghost::v1::FrameNode fn;
        fn.set_name(n.name());
        fn.set_status(n.status());
        fn.set_unschedulable(n.unschedulable());
        *fn.mutable_headroom() = n.headroom();
        f0_nodes.push_back(fn);
    }
    std::sort(f0_nodes.begin(), f0_nodes.end(), [](const ghost::v1::FrameNode& a, const ghost::v1::FrameNode& b) {
        return a.name() < b.name();
    });
    for (const auto& fn : f0_nodes) {
        *frame0->add_nodes() = fn;
    }
    
    std::vector<ghost::v1::FramePod> f0_pods;
    for (int i = 0; i < initial_topology.pods_size(); ++i) {
        const auto& p = initial_topology.pods(i);
        ghost::v1::FramePod fp;
        fp.set_id(p.id());
        fp.set_namespace_(p.namespace_());
        fp.set_name(p.name());
        fp.set_node_name(p.node_name());
        fp.set_status(p.status());
        f0_pods.push_back(fp);
    }
    std::sort(f0_pods.begin(), f0_pods.end(), [](const ghost::v1::FramePod& a, const ghost::v1::FramePod& b) {
        return a.id() < b.id();
    });
    for (const auto& fp : f0_pods) {
        *frame0->add_pods() = fp;
    }
    
    // Frame 1 (final topology)
    auto* frame1 = result.add_frames();
    frame1->set_offset_seconds(horizon);
    
    std::vector<ghost::v1::FrameNode> f1_nodes;
    for (const auto& n : nodes_state) {
        ghost::v1::FrameNode fn;
        fn.set_name(n.name());
        fn.set_status(n.status());
        fn.set_unschedulable(n.unschedulable());
        *fn.mutable_headroom() = n.headroom();
        f1_nodes.push_back(fn);
    }
    std::sort(f1_nodes.begin(), f1_nodes.end(), [](const ghost::v1::FrameNode& a, const ghost::v1::FrameNode& b) {
        return a.name() < b.name();
    });
    for (const auto& fn : f1_nodes) {
        *frame1->add_nodes() = fn;
    }
    
    std::vector<ghost::v1::FramePod> f1_pods;
    for (const auto& p : pods_state) {
        ghost::v1::FramePod fp;
        fp.set_id(p.id());
        fp.set_namespace_(p.namespace_());
        fp.set_name(p.name());
        fp.set_node_name(p.node_name());
        fp.set_status(p.status());
        f1_pods.push_back(fp);
    }
    std::sort(f1_pods.begin(), f1_pods.end(), [](const ghost::v1::FramePod& a, const ghost::v1::FramePod& b) {
        return a.id() < b.id();
    });
    for (const auto& fp : f1_pods) {
        *frame1->add_pods() = fp;
    }
    
    for (const auto& ev : simulation_events) {
        *frame1->add_events() = ev;
    }
    
    return result;
}

} // namespace ghost
