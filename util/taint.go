package util

import (
	"fmt"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Utility functions that assist with the tainting of nodes
// ----
// Taint Scheme:
// Key: ToBeRemovedByAutoscaler
// Value: time.Now().Unix()
// Effect: NoSchedule

const (
	// ToBeRemovedByAutoscalerKey specifies the key the autoscaler uses to taint nodes as MARKED
	ToBeRemovedByAutoscalerKey = "ToBeRemovedByAutoscaler"
)

// AddToBeRemovedTaint takes a k8s node and adds the ToBeRemovedByAutoscaler taint to the node
// returns the most recent update of the node that is successful
func AddToBeRemovedTaint(node *apiv1.Node, client kubernetes.Interface) (*v1.Node, error) {
	// fetch the latest version of the node to avoid conflict
	updatedNode, err := client.CoreV1().Nodes().Get(node.Name, metav1.GetOptions{})
	if err != nil || updatedNode == nil {
		return node, fmt.Errorf("failed to get node %v: %v", node.Name, err)
	}

	// check if the taint already exists
	var taintExists bool
	for _, taint := range updatedNode.Spec.Taints {
		if taint.Key == ToBeRemovedByAutoscalerKey {
			log.Debugf("%v already present on node %v", ToBeRemovedByAutoscalerKey, updatedNode.Name)
			taintExists = true
			break
		}
	}

	// don't need to re-add the taint
	if taintExists {
		return node, nil
	}

	node.Spec.Taints = append(node.Spec.Taints, apiv1.Taint{
		Key:    ToBeRemovedByAutoscalerKey,
		Value:  fmt.Sprint(time.Now().Unix()),
		Effect: v1.TaintEffectNoSchedule,
	})

	updatedNodeWithTaint, err := client.CoreV1().Nodes().Update(updatedNode)
	if err != nil || updatedNodeWithTaint == nil {
		return updatedNode, fmt.Errorf("failed to update node %v after adding taint: %v", updatedNode.Name, err)
	}

	log.Infof("Successfully added taint on node %v", updatedNodeWithTaint.Name)
	return updatedNodeWithTaint, nil
}

// GetToBeRemovedTaint returns whether the node is tainted with the ToBeRemovedByAutoscalerKey taint
// and the taint associated
func GetToBeRemovedTaint(node *apiv1.Node) (apiv1.Taint, bool) {
	for _, taint := range node.Spec.Taints {
		if taint.Key == ToBeRemovedByAutoscalerKey {
			return taint, true
		}
	}
	return apiv1.Taint{}, false
}

// GetToBeRemovedTime returns the time the node was tainted
// result will be nil if does not exist
func GetToBeRemovedTime(node *apiv1.Node) (*time.Time, error) {
	if taint, ok := GetToBeRemovedTaint(node); ok {
		timestamp, err := strconv.ParseInt(taint.Value, 10, 64)
		if err != nil {
			return nil, err
		}
		result := time.Unix(timestamp, 0)
		return &result, nil
	}
	return nil, nil
}

// DeleteToBeRemovedTaint removes the ToBeRemovedByAutoscaler taint fromt the node if it exists
// returns the latest successful update of the node
func DeleteToBeRemovedTaint(node *apiv1.Node, client kubernetes.Interface) (*apiv1.Node, error) {
	// fetch the latest version of the node to avoid conflict
	updatedNode, err := client.CoreV1().Nodes().Get(node.Name, metav1.GetOptions{})
	if err != nil || updatedNode == nil {
		return node, fmt.Errorf("failed to get node %v: %v", node.Name, err)
	}

	for i, taint := range updatedNode.Spec.Taints {
		if taint.Key == ToBeRemovedByAutoscalerKey {
			// Delete the element from the array without preserving order
			// https://github.com/golang/go/wiki/SliceTricks#delete-without-preserving-order
			updatedNode.Spec.Taints[i] = updatedNode.Spec.Taints[len(updatedNode.Spec.Taints)-1]
			updatedNode.Spec.Taints = updatedNode.Spec.Taints[:len(updatedNode.Spec.Taints)-1]

			updatedNodeWithoutTaint, err := client.CoreV1().Nodes().Update(updatedNode)
			if err != nil || updatedNodeWithoutTaint == nil {
				return updatedNode, fmt.Errorf("failed to update node %v after deleting taint: %v", updatedNode.Name, err)
			}

			log.Infof("Successfully removed taint on node %v", updatedNodeWithoutTaint.Name)
			return updatedNodeWithoutTaint, nil
		}
	}

	return updatedNode, nil
}
