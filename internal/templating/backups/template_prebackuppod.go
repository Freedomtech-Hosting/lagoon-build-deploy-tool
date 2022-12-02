package backups

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"

	"github.com/uselagoon/build-deploy-tool/internal/generator"
	"github.com/uselagoon/build-deploy-tool/internal/helpers"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	k8upv1alpha1 "github.com/vshn/k8up/api/v1alpha1"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metavalidation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"

	"sigs.k8s.io/yaml"
)

func GeneratePreBackupPod(
	lValues generator.BuildValues,
) ([]byte, error) {
	// generate the template spec

	var result []byte
	separator := []byte("---\n")

	// add the default labels
	labels := map[string]string{
		"app.kubernetes.io/managed-by": "build-deploy-tool",
		"lagoon.sh/project":            lValues.Project,
		"lagoon.sh/environment":        lValues.Environment,
		"lagoon.sh/environmentType":    lValues.EnvironmentType,
		"lagoon.sh/buildType":          lValues.BuildType,
	}

	// add the default annotations
	annotations := map[string]string{
		"lagoon.sh/version": lValues.LagoonVersion,
	}

	// add any additional labels
	additionalLabels := map[string]string{}
	additionalAnnotations := map[string]string{}
	if lValues.BuildType == "branch" {
		additionalAnnotations["lagoon.sh/branch"] = lValues.Branch
	} else if lValues.BuildType == "pullrequest" {
		additionalAnnotations["lagoon.sh/prNumber"] = lValues.PRNumber
		additionalAnnotations["lagoon.sh/prHeadBranch"] = lValues.PRHeadBranch
		additionalAnnotations["lagoon.sh/prBaseBranch"] = lValues.PRBaseBranch

	}

	// create the prebackuppods
	for _, serviceValues := range lValues.Services {
		if _, ok := preBackupPodSpecs[serviceValues.Type]; ok {
			switch lValues.Backup.K8upVersion {
			case "v1":
				prebackuppod := &k8upv1alpha1.PreBackupPod{
					TypeMeta: metav1.TypeMeta{
						Kind:       "PreBackupPod",
						APIVersion: k8upv1alpha1.GroupVersion.String(),
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("%s-prebackuppod", serviceValues.Name),
					},
					Spec: k8upv1alpha1.PreBackupPodSpec{},
				}

				prebackuppod.ObjectMeta.Labels = labels
				prebackuppod.ObjectMeta.Annotations = annotations

				var pbp bytes.Buffer
				tmpl, _ := template.New("").Funcs(funcMap).Parse(preBackupPodSpecs[serviceValues.Type])
				err := tmpl.Execute(&pbp, serviceValues)
				if err != nil {
					return nil, err
				}
				k8upPBPSpec := k8upv1alpha1.PreBackupPodSpec{}
				err = yaml.Unmarshal(pbp.Bytes(), &k8upPBPSpec)
				if err != nil {
					return nil, err
				}

				k8upPBPSpec.Pod.ObjectMeta = metav1.ObjectMeta{
					Labels: labels,
				}
				k8upPBPSpec.Pod.ObjectMeta.Labels["prebackuppod"] = serviceValues.Name
				prebackuppod.Spec = k8upPBPSpec

				for key, value := range additionalLabels {
					prebackuppod.ObjectMeta.Labels[key] = value
				}
				// add any additional annotations
				for key, value := range additionalAnnotations {
					prebackuppod.ObjectMeta.Annotations[key] = value
				}
				// validate any annotations
				if err := apivalidation.ValidateAnnotations(prebackuppod.ObjectMeta.Annotations, nil); err != nil {
					if len(err) != 0 {
						return nil, fmt.Errorf("the annotations for %s/%s are not valid: %v", "prebackuppod", serviceValues.Name, err)
					}
				}
				// validate any labels
				if err := metavalidation.ValidateLabels(prebackuppod.ObjectMeta.Labels, nil); err != nil {
					if len(err) != 0 {
						return nil, fmt.Errorf("the labels for %s/%s are not valid: %v", "prebackuppod", serviceValues.Name, err)
					}
				}

				// check length of labels
				err = helpers.CheckLabelLength(prebackuppod.ObjectMeta.Labels)
				if err != nil {
					return nil, err
				}
				// @TODO: we should review this in the future when we stop doing `kubectl apply` in the builds :)
				// marshal the resulting ingress
				prebackuppodBytes, err := yaml.Marshal(prebackuppod)
				if err != nil {
					return nil, err
				}

				pbpBytes, _ := RemoveYAML(prebackuppodBytes)
				// add the seperator to the template so that it can be `kubectl apply` in bulk as part
				// of the current build process
				restoreResult := append(separator[:], pbpBytes[:]...)
				result = append(result, restoreResult[:]...)
			case "v2":
				prebackuppod := &k8upv1.PreBackupPod{
					TypeMeta: metav1.TypeMeta{
						Kind:       "PreBackupPod",
						APIVersion: k8upv1.GroupVersion.String(),
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("%s-prebackuppod", serviceValues.Name),
					},
					Spec: k8upv1.PreBackupPodSpec{},
				}

				prebackuppod.ObjectMeta.Labels = labels
				prebackuppod.ObjectMeta.Annotations = annotations

				var pbp bytes.Buffer
				tmpl, _ := template.New("").Funcs(funcMap).Parse(preBackupPodSpecs[serviceValues.Type])
				err := tmpl.Execute(&pbp, serviceValues)
				if err != nil {
					return nil, err
				}
				k8upPBPSpec := k8upv1.PreBackupPodSpec{}
				err = yaml.Unmarshal(pbp.Bytes(), &k8upPBPSpec)
				if err != nil {
					return nil, err
				}

				k8upPBPSpec.Pod.ObjectMeta = metav1.ObjectMeta{
					Labels: labels,
				}
				k8upPBPSpec.Pod.ObjectMeta.Labels["prebackuppod"] = serviceValues.Name
				prebackuppod.Spec = k8upPBPSpec

				for key, value := range additionalLabels {
					prebackuppod.ObjectMeta.Labels[key] = value
				}
				// add any additional annotations
				for key, value := range additionalAnnotations {
					prebackuppod.ObjectMeta.Annotations[key] = value
				}
				// validate any annotations
				if err := apivalidation.ValidateAnnotations(prebackuppod.ObjectMeta.Annotations, nil); err != nil {
					if len(err) != 0 {
						return nil, fmt.Errorf("the annotations for %s/%s are not valid: %v", "prebackuppod", serviceValues.Name, err)
					}
				}
				// validate any labels
				if err := metavalidation.ValidateLabels(prebackuppod.ObjectMeta.Labels, nil); err != nil {
					if len(err) != 0 {
						return nil, fmt.Errorf("the labels for %s/%s are not valid: %v", "prebackuppod", serviceValues.Name, err)
					}
				}

				// check length of labels
				err = helpers.CheckLabelLength(prebackuppod.ObjectMeta.Labels)
				if err != nil {
					return nil, err
				}
				// @TODO: we should review this in the future when we stop doing `kubectl apply` in the builds :)
				// marshal the resulting ingress
				prebackuppodBytes, err := yaml.Marshal(prebackuppod)
				if err != nil {
					return nil, err
				}
				pbpBytes, _ := RemoveYAML(prebackuppodBytes)
				// add the seperator to the template so that it can be `kubectl apply` in bulk as part
				// of the current build process
				restoreResult := append(separator[:], pbpBytes[:]...)
				result = append(result, restoreResult[:]...)
			}
		}
	}
	return result, nil
}

// helper function to remove the creationtimestamp from the prebackuppod pod spec so that kubectl will apply without validation errors
func RemoveYAML(a []byte) ([]byte, error) {
	tmpMap := map[string]interface{}{}
	yaml.Unmarshal(a, &tmpMap)
	if _, ok := tmpMap["spec"].(map[string]interface{})["pod"].(map[string]interface{})["metadata"].(map[string]interface{})["creationTimestamp"]; ok {
		delete(tmpMap["spec"].(map[string]interface{})["pod"].(map[string]interface{})["metadata"].(map[string]interface{}), "creationTimestamp")
		b, _ := yaml.Marshal(tmpMap)
		return b, nil
	}
	return a, nil
}

var funcMap = template.FuncMap{
	"VarFix": varFix,
}

// varfix just uppercases and replaces - with _ for variable names
func varFix(s string) string {
	return fmt.Sprintf("%s", strings.ToUpper(strings.Replace(s, "-", "_", -1)))
}

// this is just the first run at doing this, once the service template generator is introduced, this will need to be re-evaluated
type PreBackupPods map[string]string

// this is just the first run at doing this, once the service template generator is introduced, this will need to be re-evaluated
var preBackupPodSpecs = PreBackupPods{
	"mariadb-dbaas": `backupCommand: |-
    /bin/sh -c "dump=$(mktemp)
    && mysqldump --max-allowed-packet=500M --events --routines --quick
    --add-locks --no-autocommit --single-transaction --no-create-db
    --no-data --no-tablespaces
    -h $BACKUP_DB_HOST
    -u $BACKUP_DB_USERNAME
    -p$BACKUP_DB_PASSWORD
    $BACKUP_DB_DATABASE
    > $dump
    && mysqldump --max-allowed-packet=500M --events --routines --quick
    --add-locks --no-autocommit --single-transaction --no-create-db
    --ignore-table=$BACKUP_DB_DATABASE.watchdog
    --no-create-info --no-tablespaces
    -h $BACKUP_DB_HOST
    -u $BACKUP_DB_USERNAME
    -p$BACKUP_DB_PASSWORD
    $BACKUP_DB_DATABASE
    >> $dump
    && cat $dump && rm $dump"
fileExtension: .{{ .Name }}.sql
pod:
  spec:
    containers:
    - args:
      - sleep
      - infinity
      env:
      - name: BACKUP_DB_HOST
        valueFrom:
          configMapKeyRef:
            key: {{ .Name | VarFix }}_HOST
            name: lagoon-env
      - name: BACKUP_DB_USERNAME
        valueFrom:
          configMapKeyRef:
            key: {{ .Name | VarFix }}_USERNAME
            name: lagoon-env
      - name: BACKUP_DB_PASSWORD
        valueFrom:
          configMapKeyRef:
            key: {{ .Name | VarFix }}_PASSWORD
            name: lagoon-env
      - name: BACKUP_DB_DATABASE
        valueFrom:
          configMapKeyRef:
            key: {{ .Name | VarFix }}_DATABASE
            name: lagoon-env
      image: imagecache.amazeeio.cloud/amazeeio/alpine-mysql-client
      imagePullPolicy: Always
      name: {{ .Name }}-prebackuppod`,
	"postgres-dbaas": `backupCommand: /bin/sh -c "PGPASSWORD=$BACKUP_DB_PASSWORD pg_dump --host=$BACKUP_DB_HOST --port=$BACKUP_DB_PORT --dbname=$BACKUP_DB_NAME --username=$BACKUP_DB_USERNAME --format=t -w"
fileExtension: .{{ .Name }}.tar
pod:
  spec:
    containers:
    - args:
      - sleep
      - infinity
      env:
      - name: BACKUP_DB_HOST
        valueFrom:
          configMapKeyRef:
            key: {{ .Name | VarFix }}_HOST
            name: lagoon-env
      - name: BACKUP_DB_USERNAME
        valueFrom:
          configMapKeyRef:
            key: {{ .Name | VarFix }}_USERNAME
            name: lagoon-env
      - name: BACKUP_DB_PASSWORD
        valueFrom:
          configMapKeyRef:
            key: {{ .Name | VarFix }}_PASSWORD
            name: lagoon-env
      - name: BACKUP_DB_DATABASE
        valueFrom:
          configMapKeyRef:
            key: {{ .Name | VarFix }}_DATABASE
            name: lagoon-env
      image: imagecache.amazeeio.cloud/uselagoon/php-8.0-cli
      imagePullPolicy: Always
      name: {{ .Name }}-prebackuppod`,
	"mongodb-dbaas": `backupCommand: /bin/sh -c "mongodump --uri=mongodb://${BACKUP_DB_USER}:${BACKUP_DB_PASSWORD}@${BACKUP_DB_HOST}:${BACKUP_DB_PORT}/${BACKUP_DB_NAME}?ssl=true&sslInsecure=true&tls=true&tlsInsecure=true --archive"
fileExtension: .{{ .Name }}.bson
pod:
  spec:
    containers:
    - args:
      - sleep
      - infinity
      env:
      - name: BACKUP_DB_HOST
        valueFrom:
          configMapKeyRef:
            key: {{ .Name | VarFix }}_HOST
            name: lagoon-env
      - name: BACKUP_DB_USERNAME
        valueFrom:
          configMapKeyRef:
            key: {{ .Name | VarFix }}_USERNAME
            name: lagoon-env
      - name: BACKUP_DB_PASSWORD
        valueFrom:
          configMapKeyRef:
            key: {{ .Name | VarFix }}_PASSWORD
            name: lagoon-env
      - name: BACKUP_DB_DATABASE
        valueFrom:
          configMapKeyRef:
            key: {{ .Name | VarFix }}_DATABASE
            name: lagoon-env
      image: imagecache.amazeeio.cloud/uselagoon/php-8.0-cli
      imagePullPolicy: Always
      name: {{ .Name }}-prebackuppod`,
	"elasticsearch": `backupCommand: /bin/sh -c "tar -cf - -C {{ .PersistentVolumePath }} ."
fileExtension: .{{ .Name }}.tar
pod:
  spec:
    affinity:
    podAffinity:
        preferredDuringSchedulingIgnoredDuringExecution:
        - podAffinityTerm:
            labelSelector:
                matchExpressions:
                - key: lagoon.sh/service
                  operator: In
                  values:
                    - {{ .Name }}
            topologyKey: kubernetes.io/hostname
            weight: 100
    containers:
    - args:
        - sleep
        - infinity
      envFrom:
        - configMapRef:
            name: lagoon-env
      image: imagecache.amazeeio.cloud/library/alpine
      imagePullPolicy: Always
      name: {{ .Name }}-prebackuppod
      volumeMounts:
        - name: {{ .PersistentVolumeName }}
          mountPath: "{{ .PersistentVolumePath }}"
    volumes:
    - name: {{ .PersistentVolumeName }}
      persistentVolumeClaim:
        claimName: {{ .PersistentVolumeName }}`,
	"opensearch": `backupCommand: /bin/sh -c "tar -cf - -C {{ .PersistentVolumePath }} ."
fileExtension: .{{ .Name }}.tar
pod:
  spec:
    affinity:
    podAffinity:
        preferredDuringSchedulingIgnoredDuringExecution:
        - podAffinityTerm:
            labelSelector:
                matchExpressions:
                - key: lagoon.sh/service
                  operator: In
                  values:
                    - {{ .Name }}
            topologyKey: kubernetes.io/hostname
            weight: 100
    containers:
    - args:
        - sleep
        - infinity
      envFrom:
        - configMapRef:
            name: lagoon-env
      image: imagecache.amazeeio.cloud/library/alpine
      imagePullPolicy: Always
      name: {{ .Name }}-prebackuppod
      volumeMounts:
        - name: {{ .PersistentVolumeName }}
          mountPath: "{{ .PersistentVolumePath }}"
    volumes:
    - name: {{ .PersistentVolumeName }}
      persistentVolumeClaim:
        claimName: {{ .PersistentVolumeName }}`,
}
