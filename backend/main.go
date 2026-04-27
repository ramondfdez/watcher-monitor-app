package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
)

// Variables globales para calcular velocidad de red
var (
	prevRxBytes    int64
	prevTxBytes    int64
	lastUpdateTime time.Time
)

// Histórico de métricas con timestamps (1 semana de datos, muestreando cada 5 minutos)
type MetricPoint struct {
	Value     float64 `json:"value"`
	Timestamp int64   `json:"timestamp"` // Unix timestamp
}

type MetricsHistory struct {
	cpuHistory       []MetricPoint
	memoryHistory    []MetricPoint
	lastSaveTime     time.Time
	mutex            sync.Mutex
}

const (
	maxHistoryPoints = 2016  // 7 días * 24 horas * 12 (cada 5 minutos) = 2016 puntos
	saveInterval     = 5 * time.Minute  // Guardar cada 5 minutos
	historyFilePath  = "/tmp/metrics_history.json"  // Archivo para persistir histórico
)

var metricsHistory = &MetricsHistory{
	cpuHistory:    make([]MetricPoint, 0, maxHistoryPoints),
	memoryHistory: make([]MetricPoint, 0, maxHistoryPoints),
	lastSaveTime:  time.Time{}, // Forzar guardado en primera llamada
}

// Estructura para serializar el histórico a JSON
type HistoryData struct {
	CPUHistory    []MetricPoint `json:"cpu_history"`
	MemoryHistory []MetricPoint `json:"memory_history"`
	LastSaveTime  int64         `json:"last_save_time"`
}

// Guardar histórico en archivo JSON
func saveHistoryToFile() error {
	metricsHistory.mutex.Lock()
	defer metricsHistory.mutex.Unlock()
	
	data := HistoryData{
		CPUHistory:    metricsHistory.cpuHistory,
		MemoryHistory: metricsHistory.memoryHistory,
		LastSaveTime:  metricsHistory.lastSaveTime.Unix(),
	}
	
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("error serializing history: %v", err)
	}
	
	err = ioutil.WriteFile(historyFilePath, jsonData, 0644)
	if err != nil {
		return fmt.Errorf("error writing history file: %v", err)
	}
	
	log.Printf("History saved to %s (%d CPU points, %d memory points)", 
		historyFilePath, len(metricsHistory.cpuHistory), len(metricsHistory.memoryHistory))
	return nil
}

// Cargar histórico desde archivo JSON
func loadHistoryFromFile() {
	file, err := ioutil.ReadFile(historyFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("No history file found, starting with empty history")
		} else {
			log.Printf("Error reading history file: %v", err)
		}
		return
	}
	
	var data HistoryData
	err = json.Unmarshal(file, &data)
	if err != nil {
		log.Printf("Error parsing history file: %v", err)
		return
	}
	
	metricsHistory.mutex.Lock()
	defer metricsHistory.mutex.Unlock()
	
	metricsHistory.cpuHistory = data.CPUHistory
	metricsHistory.memoryHistory = data.MemoryHistory
	metricsHistory.lastSaveTime = time.Unix(data.LastSaveTime, 0)
	
	log.Printf("History loaded from %s (%d CPU points, %d memory points)", 
		historyFilePath, len(metricsHistory.cpuHistory), len(metricsHistory.memoryHistory))
}

func main() {
	// Cargar histórico desde archivo (si existe)
	loadHistoryFromFile()
	
	router := mux.NewRouter()

	// CORS middleware
	router.Use(corsMiddleware)

	// Routes
	router.HandleFunc("/api/containers", listContainers).Methods("GET", "OPTIONS")
	router.HandleFunc("/api/containers/{id}/restart", restartContainer).Methods("POST", "OPTIONS")
	router.HandleFunc("/api/containers/{id}/start", startContainer).Methods("POST", "OPTIONS")
	router.HandleFunc("/api/containers/{id}/stop", stopContainer).Methods("POST", "OPTIONS")
	router.HandleFunc("/api/containers/{id}/delete", deleteContainer).Methods("POST", "OPTIONS")
	router.HandleFunc("/api/containers/{id}/logs", getContainerLogs).Methods("GET", "OPTIONS")
	router.HandleFunc("/api/stats", getSystemStats).Methods("GET", "OPTIONS")
	router.HandleFunc("/api/processes", getProcesses).Methods("GET", "OPTIONS")
	router.HandleFunc("/api/system/reboot", rebootSystem).Methods("POST", "OPTIONS")
	router.HandleFunc("/api/system/update", updateSystem).Methods("POST", "OPTIONS")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s...", port)
	log.Fatal(http.ListenAndServe(":"+port, router))
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Añadir valores al histórico (CPU y memoria juntos para mantener sincronización)
func (mh *MetricsHistory) shouldSave() bool {
	mh.mutex.Lock()
	defer mh.mutex.Unlock()
	
	now := time.Now()
	return now.Sub(mh.lastSaveTime) >= saveInterval || mh.lastSaveTime.IsZero()
}

func (mh *MetricsHistory) addMetrics(cpuValue, memoryValue float64) bool {
	mh.mutex.Lock()
	defer mh.mutex.Unlock()
	
	now := time.Now()
	// Solo guardar si pasó el intervalo o es la primera vez
	if now.Sub(mh.lastSaveTime) < saveInterval && !mh.lastSaveTime.IsZero() {
		return false
	}
	
	timestamp := now.Unix()
	mh.lastSaveTime = now
	
	// Añadir CPU
	mh.cpuHistory = append(mh.cpuHistory, MetricPoint{
		Value:     cpuValue,
		Timestamp: timestamp,
	})
	if len(mh.cpuHistory) > maxHistoryPoints {
		mh.cpuHistory = mh.cpuHistory[1:]
	}
	
	// Añadir memoria
	mh.memoryHistory = append(mh.memoryHistory, MetricPoint{
		Value:     memoryValue,
		Timestamp: timestamp,
	})
	if len(mh.memoryHistory) > maxHistoryPoints {
		mh.memoryHistory = mh.memoryHistory[1:]
	}
	
	return true
}

func (mh *MetricsHistory) getCPUHistory() []MetricPoint {
	mh.mutex.Lock()
	defer mh.mutex.Unlock()
	
	// Devolver copia para evitar race conditions
	history := make([]MetricPoint, len(mh.cpuHistory))
	copy(history, mh.cpuHistory)
	return history
}

func (mh *MetricsHistory) getMemoryHistory() []MetricPoint {
	mh.mutex.Lock()
	defer mh.mutex.Unlock()
	
	// Devolver copia para evitar race conditions
	history := make([]MetricPoint, len(mh.memoryHistory))
	copy(history, mh.memoryHistory)
	return history
}

func listContainers(w http.ResponseWriter, r *http.Request) {
	cmd := exec.Command("docker", "ps", "-a", "--format", "{{json .}}")
	var out bytes.Buffer
	cmd.Stdout = &out
	
	if err := cmd.Run(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var containerInfos []map[string]interface{}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	
	for _, line := range lines {
		if line == "" {
			continue
		}
		var container map[string]interface{}
		if err := json.Unmarshal([]byte(line), &container); err != nil {
			continue
		}
		
		// Transformar al formato esperado por el frontend
		normalized := map[string]interface{}{
			"id":      container["ID"],
			"name":    container["Names"],
			"image":   container["Image"],
			"state":   container["State"],
			"status":  container["Status"],
			"created": container["CreatedAt"],
		}
		
		containerInfos = append(containerInfos, normalized)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(containerInfos)
}

func restartContainer(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	containerID := vars["id"]

	cmd := exec.Command("docker", "restart", containerID)
	if err := cmd.Run(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "restarted"})
}

func startContainer(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	containerID := vars["id"]

	cmd := exec.Command("docker", "start", containerID)
	if err := cmd.Run(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "started"})
}

func stopContainer(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	containerID := vars["id"]

	cmd := exec.Command("docker", "stop", containerID)
	if err := cmd.Run(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "stopped"})
}

func deleteContainer(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	containerID := vars["id"]

	// Force remove container
	cmd := exec.Command("docker", "rm", "-f", containerID)
	if err := cmd.Run(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

func getContainerLogs(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	containerID := vars["id"]

	cmd := exec.Command("docker", "logs", "--tail", "100", "--timestamps", containerID)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	
	if err := cmd.Run(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write(out.Bytes())
}

func getSystemStats(w http.ResponseWriter, r *http.Request) {
	stats := make(map[string]interface{})

	// Hostname
	hostnameCmd := exec.Command("hostname")
	var hostnameOut bytes.Buffer
	hostnameCmd.Stdout = &hostnameOut
	if err := hostnameCmd.Run(); err == nil {
		stats["hostname"] = strings.TrimSpace(hostnameOut.String())
	} else {
		stats["hostname"] = "unknown"
	}

	// Uptime (from /proc/uptime)
	uptimeCmd := exec.Command("sh", "-c", "cat /proc/uptime | cut -d' ' -f1 | awk '{days=int($1/86400); hours=int(($1%86400)/3600); if(days>0) printf \"%dd %dh\", days, hours; else printf \"%dh\", hours}'")
	var uptimeOut bytes.Buffer
	uptimeCmd.Stdout = &uptimeOut
	if err := uptimeCmd.Run(); err == nil {
		uptime := strings.TrimSpace(uptimeOut.String())
		if uptime != "" {
			stats["uptime"] = uptime
		} else {
			stats["uptime"] = "N/A"
		}
	} else {
		stats["uptime"] = "N/A"
	}

	// Load Average
	loadCmd := exec.Command("sh", "-c", "cat /proc/loadavg | cut -d' ' -f1")
	var loadOut bytes.Buffer
	loadCmd.Stdout = &loadOut
	if err := loadCmd.Run(); err == nil {
		stats["load_average"] = strings.TrimSpace(loadOut.String())
	} else {
		stats["load_average"] = "0.0"
	}

	// CPU usage (approximation from load average as percentage)
	cpuCmd := exec.Command("sh", "-c", "cat /proc/loadavg | cut -d' ' -f1 | awk '{printf \"%.1f\", $1 * 25}'")
	var cpuOut bytes.Buffer
	cpuCmd.Stdout = &cpuOut
	if err := cpuCmd.Run(); err == nil {
		cpuUsage := strings.TrimSpace(cpuOut.String())
		if cpuUsage != "" {
			stats["cpu_usage"] = cpuUsage
		} else {
			stats["cpu_usage"] = "0"
		}
	} else {
		stats["cpu_usage"] = "0"
	}

	// Memory usage (using free command for accurate values)
	// Get total memory
	totalMemCmd := exec.Command("sh", "-c", "free -m | grep Mem | awk '{print $2}'")
	var totalMemOut bytes.Buffer
	totalMemCmd.Stdout = &totalMemOut
	if err := totalMemCmd.Run(); err == nil {
		totalMem := strings.TrimSpace(totalMemOut.String())
		if totalMem != "" {
			stats["memory_total_mb"] = totalMem
		} else {
			stats["memory_total_mb"] = "1024"
		}
	} else {
		stats["memory_total_mb"] = "1024"
	}

	// Get used memory (excluding buffers/cache)
	memCmd := exec.Command("sh", "-c", "free -m | grep Mem | awk '{print $3}'")
	var memOut bytes.Buffer
	memCmd.Stdout = &memOut
	if err := memCmd.Run(); err == nil {
		memUsed := strings.TrimSpace(memOut.String())
		if memUsed != "" {
			stats["memory_used_mb"] = memUsed
		} else {
			stats["memory_used_mb"] = "0"
		}
	} else {
		stats["memory_used_mb"] = "0"
	}

	// Get available memory
	memAvailCmd := exec.Command("sh", "-c", "free -m | grep Mem | awk '{print $7}'")
	var memAvailOut bytes.Buffer
	memAvailCmd.Stdout = &memAvailOut
	if err := memAvailCmd.Run(); err == nil {
		memAvail := strings.TrimSpace(memAvailOut.String())
		if memAvail != "" {
			stats["memory_available_mb"] = memAvail
		} else {
			stats["memory_available_mb"] = "0"
		}
	} else {
		stats["memory_available_mb"] = "0"
	}

	// Disk usage
	diskCmd := exec.Command("sh", "-c", "df -h / | tail -1 | awk '{print $5}' | sed 's/%//'")
	var diskOut bytes.Buffer
	diskCmd.Stdout = &diskOut
	if err := diskCmd.Run(); err == nil {
		stats["disk_usage"] = strings.TrimSpace(diskOut.String())
	} else {
		stats["disk_usage"] = "0"
	}

	// Disk space (used/total)
	diskSpaceCmd := exec.Command("sh", "-c", "df -h / | tail -1 | awk '{print $3\"/\"$2}'")
	var diskSpaceOut bytes.Buffer
	diskSpaceCmd.Stdout = &diskSpaceOut
	if err := diskSpaceCmd.Run(); err == nil {
		stats["disk_space"] = strings.TrimSpace(diskSpaceOut.String())
	} else {
		stats["disk_space"] = "0/0"
	}

	// Local IP
	ipCmd := exec.Command("sh", "-c", "hostname -i 2>/dev/null || ip route get 1 | awk '{print $7}' | head -1")
	var ipOut bytes.Buffer
	ipCmd.Stdout = &ipOut
	if err := ipCmd.Run(); err == nil {
		ip := strings.TrimSpace(ipOut.String())
		if ip != "" {
			stats["local_ip"] = strings.Split(ip, " ")[0]
		} else {
			stats["local_ip"] = "127.0.0.1"
		}
	} else {
		stats["local_ip"] = "127.0.0.1"
	}

	// Network interface
	netCmd := exec.Command("sh", "-c", "ip route | grep default | awk '{print $5}' | head -1")
	var netOut bytes.Buffer
	netCmd.Stdout = &netOut
	if err := netCmd.Run(); err == nil {
		iface := strings.TrimSpace(netOut.String())
		if iface != "" {
			stats["network_interface"] = iface
		} else {
			stats["network_interface"] = "eth0"
		}
	} else {
		stats["network_interface"] = "eth0"
	}

	// Network speed (download/upload in Mbps)
	// Get network interface
	ifaceCmd := exec.Command("sh", "-c", "ip route | grep default | awk '{print $5}' | head -1")
	var ifaceOut bytes.Buffer
	ifaceCmd.Stdout = &ifaceOut
	iface := "eth0"
	if err := ifaceCmd.Run(); err == nil {
		if i := strings.TrimSpace(ifaceOut.String()); i != "" {
			iface = i
		}
	}

	// Calculate time difference
	now := time.Now()
	timeDiff := now.Sub(lastUpdateTime).Seconds()
	if timeDiff == 0 || lastUpdateTime.IsZero() {
		timeDiff = 5.0 // Default to 5 seconds
	}
	lastUpdateTime = now

	// Download speed (RX)
	downloadCmd := exec.Command("sh", "-c", fmt.Sprintf("cat /sys/class/net/%s/statistics/rx_bytes 2>/dev/null || echo 0", iface))
	var downloadOut bytes.Buffer
	downloadCmd.Stdout = &downloadOut
	if err := downloadCmd.Run(); err == nil {
		rxBytes := strings.TrimSpace(downloadOut.String())
		if rxBytes != "" {
			var rxValue int64
			if _, err := fmt.Sscanf(rxBytes, "%d", &rxValue); err == nil {
				if prevRxBytes > 0 {
					diff := rxValue - prevRxBytes
					// Convert bytes to Mbps: bytes * 8 bits/byte / time_seconds / 1000000 bits/Mbps
					mbps := float64(diff*8) / timeDiff / 1000000
					if mbps < 0 {
						mbps = 0 // Handle counter reset
					}
					stats["download_speed"] = fmt.Sprintf("%.2f", mbps)
				} else {
					stats["download_speed"] = "0.00"
				}
				prevRxBytes = rxValue
			} else {
				stats["download_speed"] = "0.00"
			}
		} else {
			stats["download_speed"] = "0.00"
		}
	} else {
		stats["download_speed"] = "0.00"
	}

	// Upload speed (TX)
	uploadCmd := exec.Command("sh", "-c", fmt.Sprintf("cat /sys/class/net/%s/statistics/tx_bytes 2>/dev/null || echo 0", iface))
	var uploadOut bytes.Buffer
	uploadCmd.Stdout = &uploadOut
	if err := uploadCmd.Run(); err == nil {
		txBytes := strings.TrimSpace(uploadOut.String())
		if txBytes != "" {
			var txValue int64
			if _, err := fmt.Sscanf(txBytes, "%d", &txValue); err == nil {
				if prevTxBytes > 0 {
					diff := txValue - prevTxBytes
					mbps := float64(diff*8) / timeDiff / 1000000
					if mbps < 0 {
						mbps = 0 // Handle counter reset
					}
					stats["upload_speed"] = fmt.Sprintf("%.2f", mbps)
				} else {
					stats["upload_speed"] = "0.00"
				}
				prevTxBytes = txValue
			} else {
				stats["upload_speed"] = "0.00"
			}
		} else {
			stats["upload_speed"] = "0.00"
		}
	} else {
		stats["upload_speed"] = "0.00"
	}

	// Temperature (if available - works on Raspberry Pi/Orange Pi)
	tempCmd := exec.Command("sh", "-c", "cat /sys/class/thermal/thermal_zone0/temp 2>/dev/null | awk '{printf \"%.1f\", $1/1000}'")
	var tempOut bytes.Buffer
	tempCmd.Stdout = &tempOut
	if err := tempCmd.Run(); err == nil {
		temp := strings.TrimSpace(tempOut.String())
		if temp != "" {
			stats["temperature"] = temp
		} else {
			stats["temperature"] = "N/A"
		}
	} else {
		stats["temperature"] = "N/A"
	}

	// Docker containers stats
	dockerCmd := exec.Command("docker", "ps", "-q")
	var dockerOut bytes.Buffer
	dockerCmd.Stdout = &dockerOut
	if err := dockerCmd.Run(); err == nil {
		containerIDs := strings.Split(strings.TrimSpace(dockerOut.String()), "\n")
		runningCount := 0
		for _, id := range containerIDs {
			if id != "" {
				runningCount++
			}
		}
		stats["containers_running"] = runningCount
	} else {
		stats["containers_running"] = 0
	}

	// Total containers
	dockerAllCmd := exec.Command("docker", "ps", "-a", "-q")
	var dockerAllOut bytes.Buffer
	dockerAllCmd.Stdout = &dockerAllOut
	if err := dockerAllCmd.Run(); err == nil {
		allContainerIDs := strings.Split(strings.TrimSpace(dockerAllOut.String()), "\n")
		totalCount := 0
		for _, id := range allContainerIDs {
			if id != "" {
				totalCount++
			}
		}
		stats["containers_total"] = totalCount
	} else {
		stats["containers_total"] = 0
	}

	// Docker images count
	imagesCmd := exec.Command("docker", "images", "-q")
	var imagesOut bytes.Buffer
	imagesCmd.Stdout = &imagesOut
	if err := imagesCmd.Run(); err == nil {
		imageIDs := strings.Split(strings.TrimSpace(imagesOut.String()), "\n")
		imageCount := 0
		for _, id := range imageIDs {
			if id != "" {
				imageCount++
			}
		}
		stats["images_count"] = imageCount
	} else {
		stats["images_count"] = 0
	}

	// Guardar valores actuales en el histórico
	cpuValue, _ := strconv.ParseFloat(stats["cpu_usage"].(string), 64)
	
	// Calcular porcentaje de memoria
	memUsed, _ := strconv.ParseFloat(stats["memory_used_mb"].(string), 64)
	memTotal, _ := strconv.ParseFloat(stats["memory_total_mb"].(string), 64)
	memPercent := 0.0
	if memTotal > 0 {
		memPercent = (memUsed / memTotal) * 100
	}
	
	// Añadir ambas métricas juntas (mantiene sincronización)
	metricsAdded := metricsHistory.addMetrics(cpuValue, memPercent)
	
	// Si se añadió un nuevo punto, guardar en archivo
	if metricsAdded {
		go func() {
			if err := saveHistoryToFile(); err != nil {
				log.Printf("Error saving history: %v", err)
			}
		}()
	}
	
	// Añadir históricos a la respuesta
	stats["cpu_history"] = metricsHistory.getCPUHistory()
	stats["memory_history"] = metricsHistory.getMemoryHistory()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func getProcesses(w http.ResponseWriter, r *http.Request) {
	// Get top processes by CPU usage
	cmd := exec.Command("sh", "-c", "ps aux --sort=-%cpu | head -n 21 | tail -n 20 | awk '{printf \"{\\\"pid\\\":\\\"%s\\\",\\\"user\\\":\\\"%s\\\",\\\"cpu\\\":\\\"%s\\\",\\\"mem\\\":\\\"%s\\\",\\\"command\\\":\\\"%s\\\"}\\n\", $2, $1, $3, $4, substr($0, index($0,$11))}'")
	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	var processes []map[string]interface{}

	for _, line := range lines {
		if line == "" {
			continue
		}
		var proc map[string]interface{}
		if err := json.Unmarshal([]byte(line), &proc); err != nil {
			continue
		}
		processes = append(processes, proc)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(processes)
}

func rebootSystem(w http.ResponseWriter, r *http.Request) {
	// Schedule system reboot with delay to allow response
	go func() {
		// Wait 2 seconds to ensure response is sent
		time.Sleep(2 * time.Second)
		
		// Try multiple reboot methods
		// Method 1: Direct reboot (works if container has CAP_SYS_BOOT)
		if err := exec.Command("reboot").Run(); err != nil {
			log.Printf("Direct reboot failed: %v, trying with sudo...", err)
			
			// Method 2: With sudo
			if err := exec.Command("sudo", "reboot").Run(); err != nil {
				log.Printf("Sudo reboot failed: %v, trying /sbin/reboot...", err)
				
				// Method 3: Direct path
				if err := exec.Command("/sbin/reboot").Run(); err != nil {
					log.Printf("All reboot methods failed: %v", err)
				}
			}
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "El sistema se reiniciará en unos segundos",
	})
}

func updateSystem(w http.ResponseWriter, r *http.Request) {
	// Run system update in background
	go func() {
		log.Println("Starting system update...")
		cmd := exec.Command("sudo", "sh", "-c", "apt-get update && apt-get upgrade -y")
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out
		
		if err := cmd.Run(); err != nil {
			log.Printf("Update error: %s\nOutput: %s", err.Error(), out.String())
		} else {
			log.Println("System update completed successfully")
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Actualización del sistema iniciada. Revisa los logs del servidor para más detalles.",
	})
}
