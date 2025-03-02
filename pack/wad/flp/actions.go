package flp

import (
	"archive/zip"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/pkg/errors"

	"github.com/mogaika/god_of_war_browser/pack/wad"
	"github.com/mogaika/god_of_war_browser/webutils"
)

func (f *FLP) HttpAction(wrsrc *wad.WadNodeRsrc, w http.ResponseWriter, r *http.Request, action string) {
	switch action {
	case "staticlabels":
		if strings.ToUpper(r.Method) != "POST" {
			return
		}

		if err := r.ParseForm(); err != nil {
			webutils.WriteError(w, errors.Wrapf(err, "Failed to parse form"))
			return
		}

		id, err := strconv.Atoi(r.PostFormValue("id"))
		if err != nil {
			webutils.WriteError(w, errors.Wrapf(err, "Failed to parse id"))
			return
		}

		if err := f.StaticLabels[id].ParseJson([]byte(r.PostFormValue("sl"))); err != nil {
			webutils.WriteError(w, errors.Wrapf(err, "Failed to load static label"))
			return
		}

		wrsrc.Wad.UpdateTagsData(map[wad.TagId][]byte{
			wrsrc.Tag.Id: f.marshalBufferWithHeader().Bytes(),
		})
	case "importbmfont":
		if strings.ToUpper(r.Method) != "POST" {
			return
		}

		fZip, hZip, err := r.FormFile("data")
		if err != nil {
			webutils.WriteError(w, errors.Wrapf(err, "Failed to open file"))
			return
		}
		defer fZip.Close()

		scale := float32(1.0)
		if strScale := r.FormValue("scale"); len(strScale) > 0 {
			possibleScale, err := strconv.ParseFloat(strScale, 32)
			if err != nil {
				log.Printf("Error parsing scale param: %v", err)
			} else {
				scale = float32(possibleScale)
				log.Printf("Used scale %v", scale)
			}
		} else {
			log.Println("Scale parameter not provided")
		}

		zr, err := zip.NewReader(fZip, hZip.Size)
		if err != nil {
			webutils.WriteError(w, errors.Wrapf(err, "Failed to open zip reader"))
			return
		}
		if err := f.actionImportBmFont(wrsrc, zr, scale); err != nil {
			webutils.WriteError(w, errors.Wrapf(err, "Failed to import font"))
			return
		}
	case "transform":
		if strings.ToUpper(r.Method) != "POST" {
			return
		}

		if err := r.ParseForm(); err != nil {
			webutils.WriteError(w, errors.Wrapf(err, "Failed to parse form"))
			return
		}

		id, err := strconv.Atoi(r.PostFormValue("id"))
		if err != nil {
			webutils.WriteError(w, errors.Wrapf(err, "Failed to get id"))
			return
		}

		if err := json.Unmarshal([]byte(r.PostFormValue("data")), &f.Transformations[id]); err != nil {
			webutils.WriteError(w, errors.Wrapf(err, "Failed to unmarshal"))
			return
		}

		wrsrc.Wad.UpdateTagsData(map[wad.TagId][]byte{
			wrsrc.Tag.Id: f.marshalBufferWithHeader().Bytes(),
		})
	case "exportfont":
		if fnt, err := f.actionExportFont(wrsrc); err != nil {
			webutils.WriteError(w, errors.Wrapf(err, "Failed to export font"))
			return
		} else {
			webutils.WriteJsonFile(w, fnt, wrsrc.Name()+"_font")
			return
		}
	case "replacefont":
		if strings.ToUpper(r.Method) != "POST" {
			return
		}

		eff, _, err := r.FormFile("data")
		if err != nil {
			webutils.WriteError(w, errors.Wrapf(err, "Failed to open file"))
			return
		}

		defer eff.Close()

		var ef ExportedFont
		if err := json.NewDecoder(eff).Decode(&ef); err != nil {
			webutils.WriteError(w, errors.Wrapf(err, "Failed to unmarshal"))
			return
		}

		if err := f.actionReplaceFont(wrsrc, &ef); err != nil {
			webutils.WriteError(w, errors.Wrapf(err, "Failed to replace font"))
			return
		}
	case "asjson":
		webutils.WriteJsonFile(w, f, wrsrc.Name())
	case "fromjson":
		newFlp := &FLP{}
		currentFlpInstance = newFlp

		if err := webutils.ReadJsonFile(r, "data", newFlp); err != nil {
			webutils.WriteError(w, err)
		}

		// decompile scripts
		for _, d6 := range newFlp.Datas6 {
			for _, d6s1s2 := range d6.Sub1.FrameScriptLables {
				for _, d6s1s2s1 := range d6s1s2.Subs {
					if err := d6s1s2s1.Script.FromDecompiled(); err != nil {
						webutils.WriteError(w, errors.Wrapf(err, "Failed upload d6 d6s1s2s1 script"))
						return
					}
				}
			}
			for _, d6s2 := range d6.Sub2s {
				if err := d6s2.Script.FromDecompiled(); err != nil {
					webutils.WriteError(w, errors.Wrapf(err, "Failed upload d6 d6s2 script"))
					return
				}
			}
		}
		for _, d7 := range newFlp.Datas7 {
			for _, d6s1s2 := range d7.FrameScriptLables {
				for _, d6s1s2s1 := range d6s1s2.Subs {
					if err := d6s1s2s1.Script.FromDecompiled(); err != nil {
						webutils.WriteError(w, errors.Wrapf(err, "Failed upload d7 d6s1s2s1 script"))
						return
					}
				}
			}
		}
		for _, d6s1s2 := range newFlp.Data8.FrameScriptLables {
			for _, d6s1s2s1 := range d6s1s2.Subs {
				if err := d6s1s2s1.Script.FromDecompiled(); err != nil {
					webutils.WriteError(w, errors.Wrapf(err, "Failed upload d8 d6s1s2s1 script"))
					return
				}
			}
		}

		newFLPBuf := newFlp.marshalBufferWithHeader()

		// testing
		// ioutil.WriteFile("/tmp/testupload.FLP", newFLPBuf.Bytes(), 0777)
		// double check validity of file

		if _, err := NewFromData(newFLPBuf.Bytes()); err != nil {
			webutils.WriteError(w, errors.Wrapf(err, "Failed to check validity of file"))
			return
		}

		if err := wrsrc.Wad.UpdateTagsData(map[wad.TagId][]byte{
			wrsrc.Tag.Id: newFLPBuf.Bytes(),
		}); err != nil {
			webutils.WriteError(w, errors.Wrapf(err, "Failed to write tag"))
			return
		}
	}
}
