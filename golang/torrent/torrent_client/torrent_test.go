package torrent_client

import (
	//"encoding/json"
	"fmt"
	"testing"
)

var (
	client_, _ = NewTorrentService_Client("globule-ryzen.globular.cloud", "torrent.TorrentService")
)

// Test various function here.
func TestDownloadTorrent(t *testing.T) {

	// Connect to the plc client.
	// "0f80ed1a-5d3a-46f1-a3f6-c091ac259665",  "b94d0011-39a0-4bdb-9a5c-7e9abc23b26b"
	link_1 := "magnet:?xt=urn:btih:2B08B785B860C1DB692A1D2BAB8D53F4468F528E&dn=Match+Point+%282005%29+%5B1080p%5D+%5BYTS%5D+%5BYIFY%5D&tr=udp%3A%2F%2Ftracker.coppersurfer.tk%3A6969%2Fannounce&tr=udp%3A%2F%2F9.rarbg.com%3A2710%2Fannounce&tr=udp%3A%2F%2Fp4p.arenabg.com%3A1337&tr=udp%3A%2F%2Ftracker.leechers-paradise.org%3A6969&tr=udp%3A%2F%2Ftracker.internetwarriors.net%3A1337&tr=udp%3A%2F%2Ftracker.opentrackr.org%3A1337%2Fannounce&tr=udp%3A%2F%2Ftracker.zer0day.to%3A1337%2Fannounce&tr=udp%3A%2F%2Ftracker.leechers-paradise.org%3A6969%2Fannounce&tr=udp%3A%2F%2Fcoppersurfer.tk%3A6969%2Fannounce"
	// link := "magnet:?xt=urn:btih:E1E45F9A0E05753A6C30469473B813D4AFF5336C&dn=1883.S01E10.WEB.x264-PHOENiX&tr=udp%3A%2F%2Fopen.stealth.si%3A80%2Fannounce&tr=udp%3A%2F%2Ftracker.tiny-vps.com%3A6969%2Fannounce&tr=udp%3A%2F%2Ftracker.opentrackr.org%3A1337%2Fannounce&tr=udp%3A%2F%2Ftracker.torrent.eu.org%3A451%2Fannounce&tr=udp%3A%2F%2Fexplodie.org%3A6969%2Fannounce&tr=udp%3A%2F%2Ftracker.cyberia.is%3A6969%2Fannounce&tr=udp%3A%2F%2Fipv4.tracker.harry.lu%3A80%2Fannounce&tr=udp%3A%2F%2Fp4p.arenabg.com%3A1337%2Fannounce&tr=udp%3A%2F%2Ftracker.birkenwald.de%3A6969%2Fannounce&tr=udp%3A%2F%2Ftracker.moeking.me%3A6969%2Fannounce&tr=udp%3A%2F%2Fopentor.org%3A2710%2Fannounce&tr=udp%3A%2F%2Ftracker.dler.org%3A6969%2Fannounce&tr=udp%3A%2F%2F9.rarbg.me%3A2970%2Fannounce&tr=https%3A%2F%2Ftracker.foreverpirates.co%3A443%2Fannounce&tr=udp%3A%2F%2Ftracker.zer0day.to%3A1337%2Fannounce&tr=udp%3A%2F%2Ftracker.leechers-paradise.org%3A6969%2Fannounce&tr=udp%3A%2F%2Fcoppersurfer.tk%3A6969%2Fannounce"
	
	link_0 := "magnet:?xt=urn:btih:58FF685757998165BDC11A79286B13E99A084398&dn=Star.Trek.Picard.S02E01.1080p.WEB.H264-PLZPROPER&tr=http%3A%2F%2Ftracker.trackerfix.com%3A80%2Fannounce&tr=udp%3A%2F%2F9.rarbg.me%3A2980%2Fannounce&tr=udp%3A%2F%2F9.rarbg.to%3A2910%2Fannounce&tr=udp%3A%2F%2Ftracker.fatkhoala.org%3A13730%2Fannounce&tr=udp%3A%2F%2Ftracker.slowcheetah.org%3A14740%2Fannounce&tr=udp%3A%2F%2Ftracker.zer0day.to%3A1337%2Fannounce&tr=udp%3A%2F%2Ftracker.leechers-paradise.org%3A6969%2Fannounce&tr=udp%3A%2F%2Fcoppersurfer.tk%3A6969%2Fannounce"
	
	dest := "/home/dave/Videos"

	err := client_.DowloadTorrent(link_0, dest, true)
	if err != nil {
		fmt.Println("fail to download torrent ", err)
		return
	}

	//link_1 := "magnet:?xt=urn:btih:B4540808709CDADAF263F90ACA000AA65B6AC9F3&dn=A.Day.to.Die.2022.720p.WEBRip.800MB.x264-GalaxyRG&tr=udp%3A%2F%2Fopen.stealth.si%3A80%2Fannounce&tr=udp%3A%2F%2Ftracker.tiny-vps.com%3A6969%2Fannounce&tr=udp%3A%2F%2Ftracker.opentrackr.org%3A1337%2Fannounce&tr=udp%3A%2F%2Ftracker.torrent.eu.org%3A451%2Fannounce&tr=udp%3A%2F%2Fexplodie.org%3A6969%2Fannounce&tr=udp%3A%2F%2Ftracker.cyberia.is%3A6969%2Fannounce&tr=udp%3A%2F%2Fipv4.tracker.harry.lu%3A80%2Fannounce&tr=udp%3A%2F%2Fp4p.arenabg.com%3A1337%2Fannounce&tr=udp%3A%2F%2Ftracker.birkenwald.de%3A6969%2Fannounce&tr=udp%3A%2F%2Ftracker.moeking.me%3A6969%2Fannounce&tr=udp%3A%2F%2Fopentor.org%3A2710%2Fannounce&tr=udp%3A%2F%2Ftracker.dler.org%3A6969%2Fannounce&tr=udp%3A%2F%2F9.rarbg.me%3A2970%2Fannounce&tr=https%3A%2F%2Ftracker.foreverpirates.co%3A443%2Fannounce&tr=udp%3A%2F%2Ftracker.zer0day.to%3A1337%2Fannounce&tr=udp%3A%2F%2Ftracker.leechers-paradise.org%3A6969%2Fannounce&tr=udp%3A%2F%2Fcoppersurfer.tk%3A6969%2Fannounce"
	err = client_.DowloadTorrent(link_1, dest, true)
	if err != nil {
		fmt.Println("fail to download torrent ", err)
		return
	}

}

func TestGetTorrentInfos(t *testing.T) {
	
	infos, err := client_.GetTorrentInfos()
	if err != nil {
		fmt.Println("fail to download torrent ", err)
		return
	}
	fmt.Println(infos)
}