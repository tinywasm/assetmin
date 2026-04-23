package assetmin_test

import (
	"github.com/tinywasm/assetmin"
	"testing"
)

func TestReplaceRoot(t *testing.T) {
	type args struct {
		html    string
		newRoot string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Replace assets without style folder",
			args: args{
				html:    `<link rel="stylesheet" href="style.css">`,
				newRoot: "assets",
			},
			want: `<link rel="stylesheet" href="assets/style.css">`,
		},
		{
			name: "Replace multiple assets with different roots",
			args: args{
				html:    `<link rel="icon" type="image/png" href="static/favicon.png"><link rel="stylesheet" href="public/assets/style.css"><script src="public/assets/script.js"></script>`,
				newRoot: "assets",
			},
			want: `<link rel="icon" type="image/png" href="assets/favicon.png"><link rel="stylesheet" href="assets/style.css"><script src="assets/script.js"></script>`,
		},
		{
			name: "Replace static with assets",
			args: args{
				html:    `<link rel="icon" type="image/png" href="static/favicon.png">`,
				newRoot: "assets",
			},
			want: `<link rel="icon" type="image/png" href="assets/favicon.png">`,
		},
		{
			name: "Replace public/assets with css",
			args: args{
				html:    `<link rel="stylesheet" href="public/assets/style.css">`,
				newRoot: "css",
			},
			want: `<link rel="stylesheet" href="css/style.css">`,
		},
		{
			name: "Replace public/assets with js",
			args: args{
				html:    `<script src="public/assets/script.js"></script>`,
				newRoot: "js",
			},
			want: `<script src="js/script.js"></script>`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := assetmin.RewriteAssetUrls(tt.args.html, tt.args.newRoot); got != tt.want {
				t.Errorf("\nassetmin.RewriteAssetUrls():\n[%v]\nwant:\n[%v]", got, tt.want)
			}
		})
	}
}
